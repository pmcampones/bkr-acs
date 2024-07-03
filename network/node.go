package network

import (
	"broadcast_channels/crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"log/slog"
	"net"
	"sync"
)

type NodeObserver interface {
	BEBDeliver(msg []byte, sender *ecdsa.PublicKey)
}

type Node struct {
	id        string
	peersLock sync.RWMutex
	peers     map[string]*peer
	observers []NodeObserver
	config    *tls.Config
	sk        *ecdsa.PrivateKey
}

// Join Creates a new node and adds it to the network
// Receives the address of the current node and the contact of the network
// Returns the node created
func Join(id, contact, skPathname, certPathname string) *Node {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	sk, err := crypto.ReadPrivateKey(skPathname)
	if err != nil {
		slog.Error("error reading private key", "error", err)
		panic(err)
	}
	node := Node{
		id:        id,
		peersLock: sync.RWMutex{},
		peers:     make(map[string]*peer),
		observers: make([]NodeObserver, 0),
		config:    config,
		sk:        sk,
	}
	slog.Info("I am joining the network", "id", id, "contact", contact, "pk", sk.PublicKey)
	isContact := node.amIContact(contact)
	if !isContact {
		slog.Debug("I am not the contact")
		node.connectToContact(id, contact)
	} else {
		slog.Debug("I am the contact")
	}
	go node.listenConnections(id, skPathname, certPathname, isContact)
	return &node
}

func (n *Node) AddObserver(observer NodeObserver) {
	n.observers = append(n.observers, observer)
}

func (n *Node) Broadcast(msg []byte) {
	toSend := append([]byte{byte(generic)}, msg...)
	go n.processMessage(toSend, &n.sk.PublicKey)
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	for _, peer := range n.peers {
		err := send(peer.conn, toSend)
		if err != nil {
			slog.Warn("error sending to connection", "peer name", peer.name, "error", err)
		}
	}
}

func (n *Node) connectToContact(id, contact string) {
	peer, err := newOutbound(id, contact, n.config, n.sk)
	if err != nil {
		slog.Error("error connecting to contact", "error", err)
		panic(err)
	}
	slog.Debug("establishing connection with peer", "peer name", peer.name, "peer key", *peer.pk)
	go n.maintainConnection(peer, false)
}

func (n *Node) listenConnections(address, skPathname, certPathname string, amContact bool) {
	listener := n.setupTLSListener(address, skPathname, certPathname)
	defer listener.Close()
	for {
		peer, err := getInbound(listener, &n.sk.PublicKey)
		if err != nil {
			slog.Warn("error accepting connection with peer", "peer name", peer.name, "error", err)
			continue
		}
		slog.Debug("received connection from peer", "peer name", peer.name, "peer key", *peer.pk)
		go n.maintainConnection(peer, amContact)
	}
}

func (n *Node) setupTLSListener(address string, skPathname string, certPathname string) net.Listener {
	cert, err := tls.LoadX509KeyPair(certPathname, skPathname)
	if err != nil {
		slog.Error("error loading certificate and key",
			"error", err,
			"skPathname", skPathname,
			"certPathname", certPathname)
		panic(err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp", address, config)
	if err != nil {
		slog.Error("error listening on address", "address", address, "error", err)
		panic(err)
	}
	return listener
}

func (n *Node) amIContact(contact string) bool {
	return n.id == contact
}

func (n *Node) maintainConnection(peer peer, amContact bool) {
	defer n.closeConnection(peer)
	slog.Debug("maintaining connection with peer", "peer name", peer.name)
	n.updatePeers(peer)
	if amContact {
		n.sendMembership(peer)
	}
	n.readFromConnection(peer)
}

func (n *Node) updatePeers(peer peer) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	n.peers[peer.name] = &peer
}

func (n *Node) sendMembership(peer peer) {
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()
	slog.Debug("sending membership to peer", "peer name", peer.name, "membership", n.peers)
	for _, p := range n.peers {
		if p.name != peer.name {
			toSend := append([]byte{byte(membership)}, []byte(p.name)...)
			err := send(peer.conn, toSend)
			if err != nil {
				slog.Warn("error sending to connection", "peer name", peer.name, "error", err)
			}
		}
	}
}

func (n *Node) closeConnection(peer peer) {
	err := peer.conn.Close()
	if err != nil {
		slog.Warn("error closing connection", "peer name", peer.name, "error", err)
	}
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	delete(n.peers, peer.name)
}

func (n *Node) readFromConnection(peer peer) {
	for {
		msg, err := receive(peer.conn)
		if err != nil {
			slog.Error("error reading from connection", "peer name", peer.name, "error", err)
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
		for _, observer := range n.observers {
			observer.BEBDeliver(content, sender)
		}
	default:
		slog.Error("unhandled default case", "msg type", msgType, "msg content", content)
	}
}

func (n *Node) processMembershipMsg(msg []byte) {
	address := string(msg)
	outbound, err := newOutbound(n.id, address, n.config, n.sk)
	if err != nil {
		slog.Warn("error connecting to peer", "error", err)
	}
	n.maintainConnection(outbound, false)
}
