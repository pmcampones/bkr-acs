package network

import (
	"crypto/tls"
	"log"
	"unsafe"
)

type NodeObserver interface {
	BEBDeliver(msg []byte)
}

type Node struct {
	id        string
	peers     map[string]*peer
	observers []NodeObserver
	config    *tls.Config
}

// Join Creates a new node and adds it to the network
// Join Receives the address of the current node and the contact of the network
// Join Returns the node created
func Join(id, contact, skPathname, certPathname string) *Node {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	node := Node{
		id:        id,
		peers:     make(map[string]*peer),
		observers: make([]NodeObserver, 0),
		config:    config,
	}
	isContact := node.amIContact(contact)
	if !isContact {
		log.Println("I am not the contact")
		node.connectToContact(id, contact)
	} else {
		log.Println("I am the contact")
	}
	go node.listenConnections(id, skPathname, certPathname, isContact)
	return &node
}

func (n *Node) AddObserver(observer NodeObserver) {
	n.observers = append(n.observers, observer)
}

func (n *Node) Broadcast(msg []byte) {
	toSend := append([]byte{byte(generic)}, msg...)
	go n.processMessage(toSend)
	for _, peer := range n.peers {
		err := send(peer.conn, toSend)
		if err != nil {
			log.Println("Error sending to connection:", err)
		}
	}
}

func (n *Node) connectToContact(id, contact string) {
	peer, err := newOutbound(id, contact, n.config)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Establishing connection with:", peer.id)
	go n.maintainConnection(peer, false)
}

func (n *Node) listenConnections(address, skPathname, certPathname string, amContact bool) {
	cert, err := tls.LoadX509KeyPair(certPathname, skPathname)
	if err != nil {
		log.Fatal(err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp", address, config)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		peer, err := getInbound(listener)
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		log.Println("Received connection from:", peer.id)
		go n.maintainConnection(peer, amContact)
	}
}

func (n *Node) amIContact(contact string) bool {
	return n.id == contact
}

func (n *Node) maintainConnection(peer peer, amContact bool) {
	defer n.closeConnection(peer)
	log.Println("New connection with:", peer.id)
	if amContact {
		log.Println("Sending network list to:", peer.id)
		for _, p := range n.peers {
			toSend := append([]byte{byte(membership)}, []byte(p.id)...)
			err := send(peer.conn, toSend)
			if err != nil {
				log.Println("Error sending to connection:", err)
			}
		}
	}
	n.peers[peer.id] = &peer
	n.readFromConnection(peer)
}

func (n *Node) closeConnection(peer peer) {
	err := peer.conn.Close()
	if err != nil {
		log.Println("Error closing connection:", err)
	}
	delete(n.peers, peer.id)
}

func (n *Node) readFromConnection(peer peer) {
	for {
		msg, err := receive(peer.conn)
		if err != nil {
			log.Println("Error reading from connection:", err)
			return
		}
		go n.processMessage(msg)
	}
}

func (n *Node) processMessage(msg []byte) {
	msgType := messageType(bytesToInt(msg[:unsafe.Sizeof(messageType(0))]))
	content := msg[unsafe.Sizeof(messageType(0)):]
	switch msgType {
	case membership:
		n.processMembershipMsg(content)
	case generic:
		for _, observer := range n.observers {
			observer.BEBDeliver(content)
		}
	default:
		log.Println("Unhandled default case:", msgType, content)
	}
}

func (n *Node) processMembershipMsg(msg []byte) {
	address := string(msg)
	outbound, err := newOutbound(n.id, address, n.config)
	if err != nil {
		log.Println("Error connecting to peer:", err)
	}
	n.maintainConnection(outbound, false)
}
