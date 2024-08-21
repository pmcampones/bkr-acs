package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/lmittmann/tint"
	"github.com/magiconair/properties"
	"log/slog"
	"net"
	"os"
	"pace/brb"
	"pace/crypto"
	"pace/network"
	"pace/secretSharing"
	"time"
)

const dealCode = 'D'
const brbCode = 'R'

type MembershipBarrier struct {
	nodesWaiting int
	connections  []net.Conn
	barrier      chan struct{}
}

func (mb *MembershipBarrier) NotifyPeerUp(p *network.Peer) {
	mb.nodesWaiting--
	slog.Debug("Node went up", "peer", p)
	mb.connections = append(mb.connections, p.Conn)
	if mb.nodesWaiting == 0 {
		mb.barrier <- struct{}{}
	}
}

func (mb *MembershipBarrier) NotifyPeerDown(p *network.Peer) {
	slog.Warn("Node went down", "peer", p)
}

type ConcreteObserver struct {
}

func (co ConcreteObserver) BRBDeliver(msg []byte) {
	println("BRB Deliver:", string(msg))
}

func main() {
	propsPathname := flag.String("config", "config/config.properties", "pathname of the configuration file")
	address := flag.String("address", "localhost:6000", "address of the current node")
	skPathname := flag.String("sk", "sk.pem", "pathname of the private key")
	certPathname := flag.String("cert", "cert.pem", "pathname of the certificate")
	flag.Parse()
	setupLogger()
	props := properties.MustLoadFile(*propsPathname, properties.UTF8)
	contact := props.MustGetString("contact")
	node, err := makeNode(*address, contact, *skPathname, *certPathname)
	if err != nil {
		panic(fmt.Errorf("error creating node: %v", err))
	}
	dealObs := secretSharing.DealObserver{
		Code:     dealCode,
		DealChan: make(chan *secretSharing.Deal),
	}
	node.AttachMessageObserver(&dealObs)
	numNodes := props.MustGetInt("num_nodes")
	connections, err := joinNetwork(node, contact, numNodes)
	if err != nil {
		panic(err)
	}
	threshold := props.MustGetInt("threshold")
	_, err = getDeal(node, connections, threshold, *address == contact, &dealObs)
	if err != nil {
		panic(err)
	}
	testBRB(node, *skPathname)
}

func setupLogger() {
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelWarn,
			TimeFormat: time.Kitchen,
		}),
	))
	slog.Info("Set up logger")
}

func makeNode(address, contact, skPathname, certPathname string) (*network.Node, error) {
	sk, err := crypto.ReadPrivateKey(skPathname)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %v", err)
	}
	cert, err := tls.LoadX509KeyPair(certPathname, skPathname)
	if err != nil {
		return nil, fmt.Errorf("unable to read the certificate: %v", err)
	}
	node := network.NewNode(address, contact, sk, &cert)
	return node, nil
}

func joinNetwork(node *network.Node, contact string, numNodes int) ([]net.Conn, error) {
	memBarrier := MembershipBarrier{
		nodesWaiting: numNodes - 1,
		barrier:      make(chan struct{}),
	}
	node.AttachMembershipObserver(&memBarrier)
	err := node.Join(contact)
	if err != nil {
		return nil, fmt.Errorf("error joining network: %v", err)
	}
	slog.Info("Waiting for all nodes to join")
	<-memBarrier.barrier
	slog.Info("All nodes have joined")
	return memBarrier.connections, nil
}

func getDeal(node *network.Node, connections []net.Conn, threshold int, isContact bool, obs *secretSharing.DealObserver) (*secretSharing.Deal, error) {
	if isContact {
		slog.Info("Distributing Deals")
		err := secretSharing.ShareDeals(uint(threshold), node, connections, byte(dealCode), obs)
		if err != nil {
			return nil, fmt.Errorf("error sharing deals: %v", err)
		}
	}
	deal := <-obs.DealChan
	slog.Info("My deal:", deal)
	return deal, nil
}

func testBRB(node *network.Node, skPathname string) {
	sk, err := crypto.ReadPrivateKey(skPathname)
	if err != nil {
		slog.Error("Error reading private key", "error", err)
		return
	}
	observer := ConcreteObserver{}
	channel := brb.CreateBRBChannel(node, 4, 1, *sk, brbCode)
	channel.AttachObserver(observer)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		msg := []byte(input.Text())
		err := channel.BRBroadcast(msg)
		if err != nil {
			slog.Error("unable to networkChannel message", "msg", msg, "error", err)
		}
	}
}
