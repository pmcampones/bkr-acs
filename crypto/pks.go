package crypto

import (
	"bufio"
	"crypto/ecdsa"
	"fmt"
	"github.com/samber/mo"
	"log/slog"
	"os"
)

var pks = make([]ecdsa.PublicKey, 0)

var myIdx = mo.None[uint]()

func LoadPks(mapperPathname string) {
	fin, err := os.Open(mapperPathname)
	if err != nil {
		slog.Error("unable to read mapper file", "error", err)
	}
	scanner := bufio.NewScanner(fin)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		pk, err := readPublicKey(scanner.Text())
		if err != nil {
			slog.Error("unable to read public key", "error", err)
			panic(err)
		}
		pks = append(pks, *pk)
	}
}

func GetMyIdx() (uint, error) {
	if myIdx == mo.None[uint]() {
		return 0, fmt.Errorf("myIdx is not set")
	}
	return myIdx.MustGet(), nil
}

func SetMyIdx(idx uint) error {
	if myIdx != mo.None[uint]() {
		return fmt.Errorf("myIdx is already set")
	}
	myIdx = mo.Some[uint](idx)
	return nil
}

func GetPk(idx uint) *ecdsa.PublicKey {
	return &pks[idx]
}
