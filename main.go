package main

import (
	"broadcast_channels/broadcast"
	"broadcast_channels/crypto"
	"broadcast_channels/network"
	"bufio"
	"crypto/ecdsa"
	"flag"
	"github.com/lmittmann/tint"
	"github.com/magiconair/properties"
	"log/slog"
	"os"
	"time"
)

type ConcreteObserver struct {
}

func (co ConcreteObserver) BCBDeliver(msg []byte) {
	println("BCB Deliver:", string(msg))
}

func (co ConcreteObserver) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	println("BEB Deliver:", string(msg))
}

func main() {
	propsPathname := flag.String("config", "config/config.properties", "pathname of the configuration file")
	address := flag.String("address", "localhost:6000", "address of the current node")
	skPathname := flag.String("sk", "sk.pem", "pathname of the private key")
	certPathname := flag.String("cert", "cert.pem", "pathname of the certificate")
	myIdx := flag.Uint("idx", 0, "index of the public key")
	flag.Parse()
	props := properties.MustLoadFile(*propsPathname, properties.UTF8)
	contact := props.GetString("contact", "localhost:6000")
	pksMapper := props.GetString("pks", "config/pk_mapper")

	setupLogger()
	crypto.LoadPks(pksMapper)
	err := crypto.SetMyIdx(*myIdx)
	if err != nil {
		slog.Error("Error setting my index", "error", err)
		return
	}
	node := network.Join(*address, contact, *skPathname, *certPathname)
	testBCB(node, *skPathname)
	//testBEB(node)
}

func setupLogger() {
	// set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	))
	slog.Info("Set up logger")
}

func testBCB(node *network.Node, skPathname string) {
	sk, err := crypto.ReadPrivateKey(skPathname)
	if err != nil {
		slog.Error("Error reading private key", "error", err)
		return
	}
	observer := ConcreteObserver{}
	bcbChannel := broadcast.CreateChannel(node, 4, 1, *sk)
	bcbChannel.AttachObserver(observer)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		msg := []byte(input.Text())
		err := bcbChannel.BCBroadcast(msg)
		if err != nil {
			slog.Error("unable to broadcast message", "msg", msg, "error", err)
		}
	}
}

func testBRB(node *network.Node, skPathname string) {
	sk, err := crypto.ReadPrivateKey(skPathname)
	if err != nil {
		slog.Error("Error reading private key", "error", err)
		return
	}
	observer := ConcreteObserver{}
	bcbChannel := broadcast.CreateChannel(node, 4, 1, *sk)
	bcbChannel.AttachObserver(observer)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		msg := []byte(input.Text())
		err := bcbChannel.BRBroadcast(msg)
		if err != nil {
			slog.Error("unable to broadcast message", "msg", msg, "error", err)
		}
	}
}

func testBEB(node *network.Node) {
	observer := ConcreteObserver{}
	node.AddObserver(observer)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		node.Broadcast([]byte(input.Text()))
	}
}
