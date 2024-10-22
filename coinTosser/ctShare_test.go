package coinTosser

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/zk/dleq"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldUnmarshalCTShare(t *testing.T) {
	ogCT := genCTShare(t)
	data, err := ogCT.marshalBinary()
	assert.NoError(t, err)
	recovCT := emptyCTShare()
	err = recovCT.unmarshalBinary(data)
	assert.NoError(t, err)
	assert.True(t, arePointShareEquals(t, ogCT.pt, recovCT.pt))
}

func genCTShare(t *testing.T) ctShare {
	secret := group.Ristretto255.RandomScalar(rand.Reader)
	base := group.Ristretto255.RandomElement(rand.Reader)
	pt := pointShare{
		id:    group.Ristretto255.RandomScalar(rand.Reader),
		point: mulPoint(base, secret),
	}
	proof := genProof(t, secret, base)
	return ctShare{pt: pt, proof: *proof}
}

func genProof(t *testing.T, val group.Scalar, base group.Element) *dleq.Proof {
	params := dleq.Params{G: group.Ristretto255, H: crypto.SHA256, DST: []byte("ctShare_test")}
	prover := dleq.Prover{Params: params}
	commitBase := group.Ristretto255.RandomElement(rand.Reader)
	rnd := group.Ristretto255.RandomScalar(rand.Reader)
	hiddenVal := mulPoint(base, val)
	commitment := mulPoint(commitBase, val)
	proof, err := prover.ProveWithRandomness(val, base, hiddenVal, commitBase, commitment, rnd)
	assert.NoError(t, err)
	return proof
}

func areProofsEqual(t *testing.T, a, b *dleq.Proof) bool {
	aBytes, err := a.MarshalBinary()
	assert.NoError(t, err)
	bBytes, err := b.MarshalBinary()
	assert.NoError(t, err)
	return bytes.Equal(aBytes, bBytes)
}
