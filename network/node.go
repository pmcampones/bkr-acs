package network

import (
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

type NodeMessageObserver interface {
	BEBDeliver(msg []byte, sender *ecdsa.PublicKey)
}

type MembershipObserver interface {
	NotifyPeerUp(p *Peer)
	NotifyPeerDown(p *Peer)
}

type Node struct {
	id           string
	peersLock    sync.RWMutex
	peers        map[string]*Peer
	msgObservers []NodeMessageObserver
	memObservers []MembershipObserver
	config       *tls.Config
	sk           *ecdsa.PrivateKey
}

// Join Creates a new node and adds it to the network
// Receives the address of the current node and the contact of the network
// Returns the node created
func Join(id, contact string, sk *ecdsa.PrivateKey, cert *tls.Certificate) (*Node, error) {
	config := computeConfig(cert)
	node := Node{
		id:           id,
		peersLock:    sync.RWMutex{},
		peers:        make(map[string]*Peer),
		msgObservers: make([]NodeMessageObserver, 0),
		memObservers: make([]MembershipObserver, 0),
		config:       config,
		sk:           sk,
	}
	slog.Info("I am joining the network", "id", id, "contact", contact, "pk", sk.PublicKey)
	isContact := node.amIContact(contact)
	if !isContact {
		slog.Debug("I am not the contact")
		err := node.connectToContact(id, contact)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to contact: %v", err)
		}
	} else {
		slog.Debug("I am the contact")
	}
	listener := node.setupTLSListener(id)
	go node.listenConnections(listener, isContact)
	return &node, nil
}

func (n *Node) AttachMessageObserver(observer NodeMessageObserver) {
	n.msgObservers = append(n.msgObservers, observer)
}

func (n *Node) AttachMembershipObserver(observer MembershipObserver) {
	n.memObservers = append(n.memObservers, observer)
}

func (n *Node) Broadcast(msg []byte) {
	toSend := append([]byte{byte(generic)}, msg...)
	go n.processMessage(toSend, &n.sk.PublicKey)
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	for _, peer := range n.peers {
		err := send(peer.Conn, toSend)
		if err != nil {
			slog.Warn("error sending to connection", "Peer name", peer.name, "error", err)
		}
	}
}

func (n *Node) Unicast(msg []byte, c net.Conn) {
	toSend := append([]byte{byte(generic)}, msg...)
	err := send(c, toSend)
	if err != nil {
		slog.Warn("error sending to connection", "Conn", c.RemoteAddr(), "error", err)
	}
}

func (n *Node) connectToContact(id, contact string) error {
	peer, err := newOutbound(id, contact, n.config)
	if err != nil {
		return fmt.Errorf("unable to connect to contact: %v", err)
	}
	slog.Debug("establishing connection with Peer", "Peer name", peer.name, "Peer key", *peer.pk)
	go n.maintainConnection(peer, false)
	return nil
}

func (n *Node) listenConnections(listener net.Listener, amContact bool) {
	for {
		peer, err := getInbound(listener)
		if err != nil {
			slog.Warn("error accepting connection with Peer", "Peer name", peer.name, "error", err)
			continue
		}
		slog.Debug("received connection from Peer", "Peer name", peer.name, "Peer key", *peer.pk)
		go n.maintainConnection(peer, amContact)
	}
}

func (n *Node) setupTLSListener(address string) net.Listener {
	listener, err := tls.Listen("tcp", address, n.config)
	if err != nil {
		slog.Error("error listening on address", "address", address, "error", err)
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

func (n *Node) amIContact(contact string) bool {
	return n.id == contact
}

func (n *Node) maintainConnection(peer Peer, amContact bool) {
	defer n.closeConnection(peer)
	slog.Debug("maintaining connection with Peer", "Peer name", peer.name)
	n.updatePeers(peer)
	if amContact {
		err := n.sendMembership(peer)
		if err != nil {
			slog.Warn("unable to send membership to Peer", "Peer name", peer.name, "error", err)
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
	slog.Debug("sending membership to Peer", "Peer name", peer.name, "membership", n.peers)
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
		slog.Warn("error closing connection", "Peer name", peer.name, "error", err)
	}
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	delete(n.peers, peer.name)
}

func (n *Node) readFromConnection(peer Peer) {
	for {
		msg, err := receive(peer.Conn)
		if err != nil {
			slog.Error("error reading from connection", "Peer name", peer.name, "error", err)
			return
		}
		go n.processMessage(msg, peer.pk)
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
		slog.Error("unhandled default case", "msg type", msgType, "msg content", content)
	}
}

func (n *Node) processMembershipMsg(msg []byte) {
	address := string(msg)
	outbound, err := newOutbound(n.id, address, n.config)
	if err != nil {
		slog.Warn("error connecting to Peer", "error", err)
	}
	n.maintainConnection(outbound, false)
}
