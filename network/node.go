package network

import (
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"pace/utils"
	"sync"
)

var logger = utils.GetLogger(slog.LevelDebug)

type NodeMessageObserver interface {
	BEBDeliver(msg []byte, sender *ecdsa.PublicKey)
}

type MembershipObserver interface {
	NotifyPeerUp(p *Peer)
	NotifyPeerDown(p *Peer)
}

type Node struct {
	id           string
	isContact    bool
	hasJoined    bool
	peersLock    sync.RWMutex
	peers        map[string]*Peer
	msgObservers []NodeMessageObserver
	memObservers []MembershipObserver
	config       *tls.Config
	sk           *ecdsa.PrivateKey
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
		memObservers: make([]MembershipObserver, 0),
		config:       config,
		sk:           sk,
	}
	listener := node.setupTLSListener(id)
	go node.listenConnections(listener, isContact)
	return &node
}

// Join adds a new node to the network
func (n *Node) Join(contact string) error {
	if n.hasJoined {
		return fmt.Errorf("node has already joined the network")
	}
	logger.Info("I am joining the network", "contact", contact)
	if !n.isContact {
		logger.Debug("I am not the contact")
		err := n.connectToContact(contact)
		if err != nil {
			return fmt.Errorf("unable to connect to contact: %v", err)
		}
	} else {
		logger.Debug("I am the contact")
	}
	n.hasJoined = true
	return nil
}

func (n *Node) AttachMessageObserver(observer NodeMessageObserver) {
	n.msgObservers = append(n.msgObservers, observer)
}

func (n *Node) AttachMembershipObserver(observer MembershipObserver) {
	n.memObservers = append(n.memObservers, observer)
}

func (n *Node) Broadcast(msg []byte) error {
	if !n.hasJoined {
		return fmt.Errorf("node has not joined the network")
	}
	toSend := append([]byte{byte(generic)}, msg...)
	go n.processMessage(toSend, &n.sk.PublicKey)
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	logger.Debug("broadcasting message to peers", "peers", n.peers, "message", msg, "myself", n.id)
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
		return fmt.Errorf("node has not joined the network")
	}
	toSend := append([]byte{byte(generic)}, msg...)
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

func (n *Node) listenConnections(listener net.Listener, amContact bool) {
	for {
		peer, err := getInbound(listener)
		if err != nil {
			logger.Warn("error accepting connection with Peer", "Peer name", peer.name, "error", err)
			continue
		}
		logger.Debug("received connection from Peer", "Peer name", peer.name, "Peer key", *peer.Pk)
		go n.maintainConnection(peer, amContact)
	}
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
	for _, obs := range n.memObservers {
		obs.NotifyPeerUp(&peer)
	}
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
	for _, obs := range n.memObservers {
		obs.NotifyPeerDown(&peer)
	}
	err := peer.Conn.Close()
	if err != nil {
		logger.Warn("error closing connection", "Peer name", peer.name, "error", err)
	}
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	delete(n.peers, peer.name)
}

func (n *Node) readFromConnection(peer Peer) {
	for {
		msg, err := receive(peer.Conn)
		if err != nil {
			logger.Error("error reading from connection", "Peer name", peer.name, "error", err)
			return
		}
		go n.processMessage(msg, peer.Pk)
	}
}

func (n *Node) processMessage(msg []byte, sender *ecdsa.PublicKey) {
	msgType := msgType(msg[0])
	content := msg[1:]
	switch msgType {
	case membership:
		n.processMembershipMsg(content)
	case generic:
		for _, observer := range n.msgObservers {
			observer.BEBDeliver(content, sender)
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
