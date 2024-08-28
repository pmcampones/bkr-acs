package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/magiconair/properties"
	"github.com/samber/mo"
	"log/slog"
	"os"
	"pace/byzantineReliableBroadcast"
	"pace/coinTosser"
	"pace/overlayNetwork"
	"pace/utils"
)

var logger = utils.GetLogger(slog.LevelWarn)

type MembershipBarrier struct {
	nodesWaiting int
	connections  []*overlayNetwork.Peer
	barrier      chan struct{}
}

func (mb *MembershipBarrier) NotifyPeerUp(p *overlayNetwork.Peer) {
	mb.nodesWaiting--
	logger.Debug("Node went up", "peer", p)
	mb.connections = append(mb.connections, p)
	if mb.nodesWaiting == 0 {
		mb.barrier <- struct{}{}
	}
}

func (mb *MembershipBarrier) NotifyPeerDown(p *overlayNetwork.Peer) {
	logger.Warn("Node went down", "peer", p)
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
	utils.SetupDefaultLogger()
	props := properties.MustLoadFile(*propsPathname, properties.UTF8)
	err := utils.SetProps(props)
	if err != nil {
		panic(err)
	}
	contact := props.MustGetString("contact")
	node, err := makeNode(*address, contact, *skPathname, *certPathname)
	if err != nil {
		panic(fmt.Errorf("error creating node: %v", err))
	}
	numNodes := props.MustGetInt("num_nodes")
	threshold := props.MustGetInt("threshold")
	ctChannel := coinTosser.NewCoinTosserChannel(node, uint(threshold))
	_, err = joinNetwork(node, contact, numNodes)
	if err != nil {
		panic(err)
	}
	if contact == *address {
		err = ctChannel.ShareDeal()
		if err != nil {
			panic(fmt.Errorf("unable to share deal: %v", err))
		}
	}
	//testBRB(node, *skPathname)
	testCoinTosses(ctChannel)
}

func makeNode(address, contact, skPathname, certPathname string) (*overlayNetwork.Node, error) {
	sk, err := utils.ReadPrivateKey(skPathname)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %v", err)
	}
	cert, err := tls.LoadX509KeyPair(certPathname, skPathname)
	if err != nil {
		return nil, fmt.Errorf("unable to read the certificate: %v", err)
	}
	node := overlayNetwork.NewNode(address, contact, sk, &cert)
	return node, nil
}

func joinNetwork(node *overlayNetwork.Node, contact string, numNodes int) ([]*overlayNetwork.Peer, error) {
	memBarrier := MembershipBarrier{
		nodesWaiting: numNodes - 1,
		barrier:      make(chan struct{}),
	}
	node.AttachMembershipObserver(&memBarrier)
	err := node.Join(contact)
	if err != nil {
		return nil, fmt.Errorf("error joining overlayNetwork: %v", err)
	}
	logger.Info("Waiting for all nodes to join")
	<-memBarrier.barrier
	logger.Info("All nodes have joined")
	return memBarrier.connections, nil
}

func testBRB(node *overlayNetwork.Node, skPathname string) {
	sk, err := utils.ReadPrivateKey(skPathname)
	if err != nil {
		logger.Error("Error reading private key", "error", err)
		return
	}
	observer := ConcreteObserver{}
	channel := byzantineReliableBroadcast.CreateBRBChannel(node, 4, 1, *sk)
	channel.AttachObserver(observer)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		msg := []byte(input.Text())
		err := channel.BRBroadcast(msg)
		if err != nil {
			logger.Error("unable to networkChannel message", "msg", msg, "error", err)
		}
	}
}

func testCoinTosses(ctChannel *coinTosser.CTChannel) {
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		msg := []byte(input.Text())
		outputChan := make(chan mo.Result[bool])
		ctChannel.TossCoin(msg, outputChan)
		result := <-outputChan
		fmt.Printf("Received coin toss: %v\n", result.MustGet())
	}
}
