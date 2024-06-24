package crypto

import (
	"crypto/sha256"
	. "github.com/google/uuid"
)

func BytesToUUID(b []byte) UUID {
	hashVal := sha256.Sum256(b)
	return UUID(hashVal[:])
}
