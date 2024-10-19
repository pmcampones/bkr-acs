package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"pace/utils"
	"slices"
	"sync"
)

var logger = utils.GetLogger(slog.LevelWarn)

type NodeMessageObserver interface {
	BEBDeliver(msg []byte, sender *ecdsa.PublicKey)
}

type Node struct {
	id           string
	isContact    bool
	hasJoined    bool
	peersLock    sync.RWMutex
	peers        map[string]*Peer
	msgObservers []NodeMessageObserver
	memChan      chan struct{}
	config       *tls.Config
	sk           *ecdsa.PrivateKey
	listener     net.Listener
	closeChan    chan struct{}
}

func NewNode(id, contact string, sk *ecdsa.PrivateKey, cert *tls.Certificate) *Node {
	config := computeConfig(cert)
	isContact := id == contact
	node := Node{
		id:           id,
		isContact:    isContact,
		hasJoined:    false,
		peersLock:    sync.RWMutex{},
		peers:        make(map[string]*Peer),
		msgObservers: make([]NodeMessageObserver, 0),
		memChan:      make(chan struct{}),
		config:       config,
		sk:           sk,
		closeChan:    make(chan struct{}),
	}
	node.listener = node.setupTLSListener(id)
	go node.listenConnections(isContact)
	return &node
}

// Join adds a new node to the overlayNetwork
func (n *Node) Join(contact string) error {
	if n.hasJoined {
		return fmt.Errorf("node has already joined the overlayNetwork")
	}
	logger.Info("I am joining the overlayNetwork", "contact", contact)
	if !n.isContact {
		logger.Info("I am not the contact")
		err := n.connectToContact(contact)
		if err != nil {
			return fmt.Errorf("unable to connect to contact: %v", err)
		}
	} else {
		logger.Info("I am the contact")
	}
	n.hasJoined = true
	return nil
}

func (n *Node) AttachMessageObserver(observer NodeMessageObserver) {
	n.msgObservers = append(n.msgObservers, observer)
}

func (n *Node) Broadcast(msg []byte) error {
	if !n.hasJoined {
		return fmt.Errorf("node has not joined the overlayNetwork")
	}
	toSend := append([]byte{byte(generic)}, msg...)
	go n.processMessage(toSend, &n.sk.PublicKey)
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	logger.Debug("broadcasting message to peers", "peers", n.peers, "message", string(msg), "myself", n.id)
	for _, peer := range n.peers {
		err := send(peer.Conn, toSend)
		if err != nil {
			logger.Warn("error sending to connection", "Peer name", peer.name, "error", err)
		}
	}
	return nil
}

func (n *Node) Unicast(msg []byte, c net.Conn) error {
	if !n.hasJoined {
		return fmt.Errorf("node has not joined the overlayNetwork")
	}
	toSend := append([]byte{byte(generic)}, msg...)
	logger.Debug("unicasting message to connection", "Conn", c.RemoteAddr(), "message", string(msg), "myself", n.id)
	err := send(c, toSend)
	if err != nil {
		logger.Warn("error sending to connection", "Conn", c.RemoteAddr(), "error", err)
	}
	return nil
}

func (n *Node) connectToContact(contact string) error {
	peer, err := newOutbound(n.id, contact, n.config)
	if err != nil {
		return fmt.Errorf("unable to connect to contact: %v", err)
	}
	logger.Debug("establishing connection with Peer", "Peer name", peer.name, "Peer key", *peer.Pk)
	go n.maintainConnection(peer, false)
	return nil
}

func (n *Node) listenConnections(amContact bool) {
	for {
		peer, err := getInbound(n.listener)
		if err != nil {
			if isListenerClosed(err) {
				logger.Info("closing listener")
				break
			} else {
				logger.Warn("error accepting connection with Peer", "Peer name", peer.name, "error", err)
				continue
			}
		}
		logger.Debug("received connection from Peer", "Peer name", peer.name, "Peer key", *peer.Pk)
		go n.maintainConnection(peer, amContact)
	}
	n.closeChan <- struct{}{}
}

func isListenerClosed(err error) bool {
	var lce listenerCloseError
	return errors.As(err, &lce)
}

func (n *Node) setupTLSListener(address string) net.Listener {
	listener, err := tls.Listen("tcp", address, n.config)
	if err != nil {
		logger.Error("error listening on address", "address", address, "error", err)
		panic(err)
	}
	return listener
}

func computeConfig(cert *tls.Certificate) *tls.Config {
	config := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{*cert},
		ClientAuth:         tls.RequestClientCert,
	}
	return config
}

func (n *Node) maintainConnection(peer Peer, amContact bool) {
	defer n.closeConnection(peer)
	logger.Debug("maintaining connection with Peer", "Peer name", peer.name)
	n.updatePeers(peer)
	if amContact {
		err := n.sendMembership(peer)
		if err != nil {
			logger.Warn("unable to send membership to Peer", "Peer name", peer.name, "error", err)
			return
		}
	}
	go func() { n.memChan <- struct{}{} }()
	n.readFromConnection(peer)
}

func (n *Node) updatePeers(peer Peer) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	n.peers[peer.name] = &peer
}

func (n *Node) sendMembership(peer Peer) error {
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	logger.Debug("sending membership to Peer", "Peer name", peer.name, "membership", n.peers)
	for _, p := range n.peers {
		if p.name != peer.name {
			toSend := append([]byte{byte(membership)}, []byte(p.name)...)
			err := send(peer.Conn, toSend)
			if err != nil {
				return fmt.Errorf("unable to send membership to Peer: %v", err)
			}
		}
	}
	return nil
}

func (n *Node) closeConnection(peer Peer) {
	err := peer.Conn.Close()
	if err != nil {
		logger.Warn("error closing connection", "Peer name", peer.name, "error", err)
	}
	n.forgetPeer(peer)
}

func (n *Node) forgetPeer(peer Peer) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	delete(n.peers, peer.name)
}

func (n *Node) closeAllConnections() {
	logger.Info("closing all connections")
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	for _, peer := range n.peers {
		err := peer.Conn.Close()
		if err != nil {
			logger.Warn("error closing connection", "Peer name", peer.name, "error", err)
		}
	}
	pNames := slices.Collect(maps.Keys(n.peers))
	for _, p := range pNames {
		delete(n.peers, p)
		logger.Debug("peer deleted", "peer name", p)
	}
}

func (n *Node) readFromConnection(peer Peer) {
	for {
		msg, err := receive(peer.Conn)
		if err != nil {
			if isConnectionClosed(err) {
				logger.Debug("connection closed", "Peer name", peer.name)
				n.forgetPeer(peer)
				return
			} else {
				logger.Warn("error reading from connection", "Peer name", peer.name, "error", err)
				continue
			}
		}
		go n.processMessage(msg, peer.Pk)
	}
}

func isConnectionClosed(err error) bool {
	var cce connCloseError
	return errors.As(err, &cce)
}

func (n *Node) processMessage(msg []byte, sender *ecdsa.PublicKey) {
	msgType := msgType(msg[0])
	content := msg[1:]
	switch msgType {
	case membership:
		n.processMembershipMsg(content)
	case generic:
		for _, observer := range n.msgObservers {
			go func() { observer.BEBDeliver(content, sender) }()
		}
	default:
		logger.Error("unhandled default case", "msg type", msgType, "msg content", content)
	}
}

func (n *Node) processMembershipMsg(msg []byte) {
	address := string(msg)
	outbound, err := newOutbound(n.id, address, n.config)
	if err != nil {
		logger.Warn("error connecting to Peer", "error", err)
	}
	n.maintainConnection(outbound, false)
}

func (n *Node) GetPk() *ecdsa.PublicKey {
	return &n.sk.PublicKey
}

func (n *Node) GetPeers() []*Peer {
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	peers := make([]*Peer, 0, len(n.peers))
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	return peers
}

func (n *Node) GetMembershipChan() <-chan struct{} {
	return n.memChan
}

func (n *Node) Disconnect() error {
	err := n.listener.Close()
	n.closeAllConnections()
	<-n.closeChan
	return err
}
