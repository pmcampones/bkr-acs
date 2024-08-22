package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	. "github.com/google/uuid"
	"log"
	"math/big"
	"os"
	"time"
)

func BytesToUUID(b []byte) UUID {
	hashVal := sha256.Sum256(b)
	return UUID(hashVal[:])
}

func ReadPrivateKey(skPathname string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(skPathname)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func SerializePublicKey(pk *ecdsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pk)
}

// MakeSelfSignedCert Code shamelessly stolen from https://golang.org/src/crypto/tls/generate_cert.go
func MakeSelfSignedCert(sk *ecdsa.PrivateKey) (*tls.Certificate, error) {
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

func GenKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func GenPK() (*ecdsa.PublicKey, error) {
	sk, err := GenKey()
	if err != nil {
		return nil, err
	}
	return &sk.PublicKey, nil
}
