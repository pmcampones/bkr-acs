package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"math/big"
	"testing"
	"time"
)

func GetTestNode(t *testing.T, address, contact string) *Node {
	node, err := NewNode(address, contact)
	assert.NoError(t, err)
	return node
}

func InitializeNodes(t *testing.T, nodes []*Node) {
	for _, n := range nodes {
		err := n.Join()
		assert.NoError(t, err)
	}
	for _, n := range nodes {
		n.WaitForPeers(uint(len(nodes) - 1))
	}
}

// makeSelfSignedCert Code shamelessly stolen from https://golang.org/src/crypto/tls/generate_cert.go
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
