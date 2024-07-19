package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/magiconair/properties/assert"
	"log"
	"math/big"
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
			t.Errorf("unable to get inbound peer: %v", err)
			return
		}
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

// Code shamelessly stolen from https://golang.org/src/crypto/tls/generate_cert.go
func makeSelfSignedCert(sk *ecdsa.PrivateKey) (*tls.Certificate, error) {
	notBefore := time.Now()
	notAfter := time.Now().Add(365 * 24 * time.Hour)
	keyUsage := x509.KeyUsageDigitalSignature
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &sk.PublicKey, sk)
	if err != nil {
		return nil, fmt.Errorf("unable to create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse certificate: %v", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  sk,
		Leaf:        cert,
	}, nil
}
