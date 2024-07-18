package crypto

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	. "github.com/google/uuid"
	"os"
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
