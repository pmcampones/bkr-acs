package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"log/slog"
	"net"
	"pace/utils"
	"sync"
)

var logger = utils.GetLogger(slog.LevelWarn)

type nodeMessageObserver interface {
	bebDeliver(msg []byte, sender *ecdsa.PublicKey)
}

type Node struct {
	address      string
	contact      string
	hasJoined    bool
	peersLock    sync.RWMutex
	peers        []*peer
	msgObservers []nodeMessageObserver
	memChan      chan struct{}
	config       *tls.Config
	sk           *ecdsa.PrivateKey
	listener     net.Listener
	closeChan    chan struct{}
}

func NewNode(address, contact string) (*Node, error) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("unable to generate secret key: %v", err)
	}
	cert, err := makeSelfSignedCert(sk)
	if err != nil {
		return nil, fmt.Errorf("unable to make self-signed certificate: %v", err)
	}
	config := computeConfig(cert)
	isContact := address == contact
	node := Node{
		address:      address,
		contact:      contact,
		hasJoined:    false,
		peersLock:    sync.RWMutex{},
		peers:        make([]*peer, 0),
		msgObservers: make([]nodeMessageObserver, 0),
		memChan:      make(chan struct{}),
		config:       config,
		sk:           sk,
		closeChan:    make(chan struct{}, 1),
	}
	node.listener = node.setupTLSListener(address)
	go node.listenConnections(isContact)
	return &node, nil
}

// Join adds a new node to the overlayNetwork
func (n *Node) Join() error {
	if n.hasJoined {
		return fmt.Errorf("node has already joined the overlayNetwork")
	}
	logger.Info("I am joining the overlayNetwork", "contact", n.contact)
	if !(n.address == n.contact) {
		logger.Info("I am not the contact")
		err := n.connectToContact()
		if err != nil {
			return fmt.Errorf("unable to connect to contact: %v", err)
		}
	} else {
		logger.Info("I am the contact")
	}
	n.hasJoined = true
	return nil
}

func (n *Node) attachMessageObserver(observer nodeMessageObserver) {
	n.msgObservers = append(n.msgObservers, observer)
}

func (n *Node) unicast(msg []byte, c net.Conn) error {
	if !n.hasJoined {
		return fmt.Errorf("node has not joined the overlayNetwork")
	}
	toSend := append([]byte{byte(generic)}, msg...)
	logger.Debug("unicasting message to connection", "conn", c.RemoteAddr(), "message", string(msg), "myself", n.address)
	err := send(c, toSend)
	if err != nil {
		logger.Warn("error sending to connection", "conn", c.RemoteAddr(), "error", err)
	}
	return nil
}

func (n *Node) unicastSelf(msg []byte) error {
	if !n.hasJoined {
		return fmt.Errorf("node has not joined the overlayNetwork")
	}
	toSend := append([]byte{byte(generic)}, msg...)
	go n.processMessage(toSend, &n.sk.PublicKey)
	return nil
}

func (n *Node) connectToContact() error {
	peer, err := newOutbound(n.address, n.contact, n.config)
	if err != nil {
		return fmt.Errorf("unable to connect to contact: %v", err)
	}
	logger.Debug("establishing connection with peer", "peer name", peer.name, "peer key", *peer.pk)
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
				logger.Warn("error accepting connection with peer", "peer name", peer.name, "error", err)
				continue
			}
		}
		logger.Debug("received connection from peer", "peer name", peer.name, "peer key", *peer.pk)
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

func (n *Node) maintainConnection(peer peer, amContact bool) {
	defer n.closeConnection(peer)
	logger.Debug("maintaining connection with peer", "peer name", peer.name)
	n.updatePeers(peer)
	if amContact {
		err := n.sendMembership(peer)
		if err != nil {
			logger.Warn("unable to send membership to peer", "peer name", peer.name, "error", err)
			return
		}
	}
	go func() { n.memChan <- struct{}{} }()
	n.readFromConnection(peer)
}

func (n *Node) updatePeers(peer peer) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	n.peers = append(n.peers, &peer)
}

func (n *Node) sendMembership(peer peer) error {
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	logger.Debug("sending membership to peer", "peer name", peer.name, "membership", n.peers)
	for _, p := range n.peers {
		if p.name != peer.name {
			toSend := append([]byte{byte(membership)}, []byte(p.name)...)
			err := send(peer.conn, toSend)
			if err != nil {
				return fmt.Errorf("unable to send membership to peer: %v", err)
			}
		}
	}
	return nil
}

func (n *Node) closeConnection(peer peer) {
	err := peer.conn.Close()
	if err != nil {
		logger.Warn("error closing connection", "peer name", peer.name, "error", err)
	}
	n.forgetPeer(peer)
}

func (n *Node) forgetPeer(rem peer) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	n.peers = lo.Filter(n.peers, func(p *peer, _ int) bool { return rem.name != p.name })
}

func (n *Node) closeAllConnections() {
	logger.Info("closing all connections")
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	for _, peer := range n.peers {
		err := peer.conn.Close()
		if err != nil {
			logger.Warn("error closing connection", "peer name", peer.name, "error", err)
		}
	}
	n.peers = make([]*peer, 0)
}

func (n *Node) readFromConnection(peer peer) {
	for {
		msg, err := receive(peer.conn)
		if err != nil {
			if isConnectionClosed(err) {
				logger.Debug("connection closed", "peer name", peer.name)
				n.forgetPeer(peer)
				return
			} else {
				logger.Warn("error reading from connection", "peer name", peer.name, "error", err)
				continue
			}
		}
		go n.processMessage(msg, peer.pk)
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
			go func() { observer.bebDeliver(content, sender) }()
		}
	default:
		logger.Error("unhandled default case", "msg type", msgType, "msg content", string(content))
	}
}

func (n *Node) processMembershipMsg(msg []byte) {
	address := string(msg)
	outbound, err := newOutbound(n.address, address, n.config)
	if err != nil {
		logger.Warn("error connecting to peer", "error", err)
	}
	n.maintainConnection(outbound, false)
}

func (n *Node) getPeers() []*peer {
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	peers := make([]*peer, 0, len(n.peers))
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	return peers
}

func (n *Node) GetId() (uuid.UUID, error) {
	pk := n.sk.PublicKey
	id, err := utils.PkToUUID(&pk)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("unable to extract ID from public key")
	}
	return id, nil
}

func (n *Node) GetPeerIds() ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0)
	for address, p := range n.peers {
		id, err := utils.PkToUUID(p.pk)
		if err != nil {
			return nil, fmt.Errorf("unable to extract ID from %v public key", address)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (n *Node) Close() error {
	err := n.listener.Close()
	n.closeAllConnections()
	<-n.closeChan
	return err
}
