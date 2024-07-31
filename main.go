package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/lmittmann/tint"
	"github.com/magiconair/properties"
	"log/slog"
	"os"
	"pace/brb"
	"pace/crypto"
	"pace/network"
	"time"
)

type ConcreteObserver struct {
}

func (co ConcreteObserver) BCBDeliver(msg []byte) {
	println("BCB Deliver:", string(msg))
}

func (co ConcreteObserver) BEBDeliver(msg []byte, _ *ecdsa.PublicKey) {
	println("BEB Deliver:", string(msg))
}

func main() {
	propsPathname := flag.String("config", "config/config.properties", "pathname of the configuration file")
	address := flag.String("address", "localhost:6000", "address of the current node")
	skPathname := flag.String("sk", "sk.pem", "pathname of the private key")
	certPathname := flag.String("cert", "cert.pem", "pathname of the certificate")
	flag.Parse()
	props := properties.MustLoadFile(*propsPathname, properties.UTF8)
	contact := props.GetString("contact", "localhost:6000")
	setupLogger()
	node, err := makeNode(*address, contact, *skPathname, *certPathname)
	if err != nil {
		slog.Error("Error joining network", "error", err)
		return
	}
	testBRB(node, *skPathname)
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
	node, err := network.Join(address, contact, sk, &cert)
	if err != nil {
		return nil, fmt.Errorf("unable to join the network: %v", err)
	}
	return node, nil
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

func testBRB(node *network.Node, skPathname string) {
	sk, err := crypto.ReadPrivateKey(skPathname)
	if err != nil {
		slog.Error("Error reading private key", "error", err)
		return
	}
	observer := ConcreteObserver{}
	channel := brb.CreateChannel(node, 4, 1, *sk)
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
