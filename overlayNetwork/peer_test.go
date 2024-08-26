package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"github.com/magiconair/properties/assert"
	"pace/utils"
	"testing"
	"time"
)

func TestShouldEstablishCorrectConnection(t *testing.T) {
	serverConfig, serverSk, err := makeTLSConfig()
	if err != nil {
		t.Fatalf("unable to create server config: %v", err)
	}
	clientConfig, clientSk, err := makeTLSConfig()
	if err != nil {
		t.Fatalf("unable to create client config: %v", err)
	}
	server := "localhost:6000"
	client := "localhost:6001"
	go func() {
		listener, err := tls.Listen("tcp", server, serverConfig)
		if err != nil {
			t.Errorf("unable to listen on address: %v", err)
			return
		}
		inboundPeer, err := getInbound(listener)
		if err != nil {
			t.Errorf("unable to get inbound Peer: %v", err)
			return
		}
		assert.Equal(t, inboundPeer.name, client)
		assert.Equal(t, *inboundPeer.Pk, clientSk.PublicKey)
	}()
	time.Sleep(1 * time.Second)
	outboundPeer, err := newOutbound(client, server, clientConfig)
	if err != nil {
		t.Fatalf("unable to create outbound Peer: %v", err)
	}
	assert.Equal(t, outboundPeer.name, server)
	assert.Equal(t, *outboundPeer.Pk, serverSk.PublicKey)
}

func makeTLSConfig() (*tls.Config, *ecdsa.PrivateKey, error) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate key: %v", err)
	}
	cert, err := utils.MakeSelfSignedCert(sk)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create cert: %v", err)
	}
	fmt.Println(sk.PublicKey)
	fmt.Println(cert.Leaf.PublicKey)
	config := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{*cert},
		ClientAuth:         tls.RequestClientCert,
	}
	return config, sk, nil
}
