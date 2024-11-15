package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestShouldEstablishCorrectConnection(t *testing.T) {
	serverConfig, serverSk, err := makeTLSConfig()
	assert.NoError(t, err)
	clientConfig, clientSk, err := makeTLSConfig()
	assert.NoError(t, err)
	server := "localhost:6000"
	client := "localhost:6001"
	go func() {
		listener, err := tls.Listen("tcp", server, serverConfig)
		assert.NoError(t, err)
		inboundPeer, err := getInbound(listener)
		assert.NoError(t, err)
		assert.Equal(t, inboundPeer.name, client)
		assert.Equal(t, *inboundPeer.pk, clientSk.PublicKey)
	}()
	time.Sleep(1 * time.Second)
	outboundPeer, err := newOutbound(client, server, clientConfig)
	if err != nil {
		t.Fatalf("unable to create outbound peer: %v", err)
	}
	assert.Equal(t, outboundPeer.name, server)
	assert.Equal(t, *outboundPeer.pk, serverSk.PublicKey)
}

func makeTLSConfig() (*tls.Config, *ecdsa.PrivateKey, error) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate key: %v", err)
	}
	cert, err := makeSelfSignedCert(sk)
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
