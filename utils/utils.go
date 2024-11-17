package utils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"github.com/cloudflare/circl/group"
	. "github.com/google/uuid"
	"github.com/lmittmann/tint"
	"log/slog"
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

func SerializePublicKey(pk *ecdsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pk)
}

func PkToUUID(pk *ecdsa.PublicKey) (UUID, error) {
	pkBytes, err := SerializePublicKey(pk)
	if err != nil {
		return UUID{}, fmt.Errorf("unable to serialize public key: %v", err)
	}
	return BytesToUUID(pkBytes), nil
}

func GetLogger(prefix string, level slog.Level) *slog.Logger {
	coloredHandler := tint.NewHandler(os.Stdout, &tint.Options{
		Level:      level,
		TimeFormat: time.Kitchen,
	})
	prefixedHandler := NewPrefixedHandler(prefix, coloredHandler)
	return slog.New(prefixedHandler)
}

func GetScalarSize() (int, error) {
	scalar := group.Ristretto255.NewScalar()
	scalarBytes, err := scalar.MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("unable to marshal scalar: %v", err)
	}
	return len(scalarBytes), nil
}
