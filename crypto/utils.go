package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
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

func readPublicKey(pkPathname string) (*ecdsa.PublicKey, error) {
	data, err := os.ReadFile(pkPathname)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	return DeserializePublicKey(block.Bytes)
}

func SerializePublicKey(pk *ecdsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pk)
}

func DeserializePublicKey(data []byte) (*ecdsa.PublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, err
	}
	return pub.(*ecdsa.PublicKey), nil
}

func Sign(sk *ecdsa.PrivateKey, msg []byte) ([]byte, error) {
	hash := sha256.Sum256(msg)
	return sk.Sign(rand.Reader, hash[:], nil)
}

func Verify(pk *ecdsa.PublicKey, msg []byte, sig []byte) bool {
	hash := sha256.Sum256(msg)
	return ecdsa.VerifyASN1(pk, hash[:], sig)
}
