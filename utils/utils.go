package utils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/cloudflare/circl/group"
	. "github.com/google/uuid"
	"github.com/lmittmann/tint"
	"log"
	"log/slog"
	"math/big"
	"os"
	"time"
	"unsafe"
)

func BytesToUUID(b []byte) UUID {
	hashVal := sha256.Sum256(b)
	return UUID(hashVal[:])
}

func ExtractIdFromMessage(reader *bytes.Reader) (UUID, error) {
	idLen := unsafe.Sizeof(UUID{})
	idBytes := make([]byte, idLen)
	num, err := reader.Read(idBytes)
	if err != nil {
		return Nil, fmt.Errorf("unable to read idBytes from message during instance idBytes computation: %v", err)
	} else if num != int(idLen) {
		return Nil, fmt.Errorf("unable to read idBytes from message during instance idBytes computation: read %d bytes, expected %d", num, idLen)
	}
	id, err := FromBytes(idBytes)
	if err != nil {
		return Nil, fmt.Errorf("unable to convert idBytes to UUID during instance idBytes computation: %v", err)
	}
	return id, nil
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

func DeserializePublicKey(data []byte) (*ecdsa.PublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse public key from data: %v", err)
	}
	return pub.(*ecdsa.PublicKey), nil
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

func PkToUUID(pk *ecdsa.PublicKey) (UUID, error) {
	pkBytes, err := SerializePublicKey(pk)
	if err != nil {
		return UUID{}, fmt.Errorf("unable to serialize public key: %v", err)
	}
	return BytesToUUID(pkBytes), nil
}

func SetupDefaultLogger() {
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	))
	slog.Info("Set up logger")
}

func GetLogger(level slog.Level) *slog.Logger {
	return slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
		}),
	)
}

func GetCode(propName string) byte {
	props, err := GetProps()
	if err != nil {
		panic(fmt.Errorf("unable to get code from properties: %v", err))
	}
	return []byte(props.MustGet(propName))[0]
}

func HashToBool(seed []byte) bool {
	hash := sha256.Sum256(seed)
	return hash[0]%2 == 0
}

func GetScalarSize() (int, error) {
	scalar := group.Ristretto255.NewScalar()
	scalarBytes, err := scalar.MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("unable to marshal scalar: %v", err)
	}
	return len(scalarBytes), nil
}

func GetElementSize() (int, error) {
	element := group.Ristretto255.NewElement()
	elementBytes, err := element.MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("unable to marshal element: %v", err)
	}
	return len(elementBytes), nil
}
