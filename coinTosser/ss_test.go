package coinTosser

import (
	"crypto"
	"crypto/rand"
	"encoding/hex"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"github.com/cloudflare/circl/zk/dleq"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRecoverSecret(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(50)
	threshold := uint(20)
	secret := g.NewScalar().SetUint64(1234567890)
	shares := shareSecret(threshold, nodes, secret)
	assert.Equal(t, int(nodes), len(shares))
	recov1, err := secretsharing.Recover(threshold, shares[:threshold+1])
	assert.NoError(t, err)
	assert.Equal(t, secret, recov1)
	recov2, err := secretsharing.Recover(threshold, shares[1:threshold+2])
	assert.NoError(t, err)
	assert.Equal(t, secret, recov2)
}

func TestSSWithPoints(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(50)
	threshold := uint(20)
	secret := g.NewScalar().SetUint64(1234567890)
	base := g.HashToElement([]byte("base"), []byte("ss_tests"))
	shares := shareSecret(threshold, nodes, secret)
	pointSecret := g.Identity().Mul(base, secret)
	hiddenShares := lo.Map(shares, func(share secretsharing.Share, _ int) pointShare { return shareToPoint(share, base) })
	recov1 := recoverSecretFromPoints(hiddenShares[:])
	ptSecBytes, err := pointSecret.MarshalBinary()
	assert.NoError(t, err)
	recSecBytes, err := recov1.MarshalBinary()
	assert.NoError(t, err)
	assert.Equal(t, recSecBytes, ptSecBytes)
}

// TestDLEquivalence tests the equivalence of the DLEQ implementation in zk/dleq and the one in coinTosser.
// Taken from: https://asecuritysite.com/dleq/circl_dl
func TestDLEquivalence(t *testing.T) {
	g := group.Ristretto255
	dst := "Zero"
	params := dleq.Params{G: g, H: crypto.SHA256, DST: []byte(dst)}

	prover := dleq.Prover{Params: params}
	verifier := dleq.Verifier{Params: params}

	x := g.RandomScalar(rand.Reader)
	G := g.RandomElement(rand.Reader)
	A := g.NewElement().Mul(G, x)

	H := g.RandomElement(rand.Reader)
	B := g.NewElement().Mul(H, x)

	t.Logf("Value to prove: %v\n", x)
	t.Logf("Public value (A):\n%v\n", A)
	t.Logf("Public value (B):\n%v\n", B)
	t.Logf("Domain seperation: %s\n\n", dst)

	rr := g.HashToScalar([]byte("randomness"), []byte("ss_test"))
	proof, _ := prover.ProveWithRandomness(x, G, A, H, B, rr)
	p, _ := proof.MarshalBinary()
	t.Logf("\nProof with randomness: %+v\n", hex.EncodeToString(p))
	rtn := verifier.Verify(G, A, H, B, proof)

	t.Logf("Verification: %+v\n", rtn)
}

func TestDLEquivalenceManyShares(t *testing.T) {
	nodes := uint(50)
	threshold := uint(20)
	g := group.Ristretto255
	dst := "Zero"
	params := dleq.Params{G: g, H: crypto.SHA256, DST: []byte(dst)}
	prover := dleq.Prover{Params: params}
	verifier := dleq.Verifier{Params: params}
	secret := g.NewScalar().SetUint64(1234567890)
	commitBase := g.HashToElement([]byte("commit"), []byte("point"))
	randomBase := g.HashToElement([]byte("randomBase"), []byte("1"))
	shares := shareSecret(threshold, nodes, secret)
	rnd := g.HashToScalar([]byte("randomVal"), []byte("1"))
	for _, share := range shares {
		hiddenShare := g.NewElement().Mul(randomBase, share.Value)
		commitment := g.NewElement().Mul(commitBase, share.Value)
		proof, err := prover.ProveWithRandomness(share.Value, randomBase, hiddenShare, commitBase, commitment, rnd)
		assert.NoError(t, err)
		res := verifier.Verify(randomBase, hiddenShare, commitBase, commitment, proof)
		assert.Equal(t, res, true)
	}
}

func TestHashPointToBool(t *testing.T) {
	g := group.Ristretto255
	point := g.HashToElement([]byte("base"), []byte("point"))
	b, err := hashPointToBool(point)
	assert.NoError(t, err)
	t.Log(b)
}

func TestMarshalAndUnmarshal(t *testing.T) {
	g := group.Ristretto255
	ogShare := secretsharing.Share{ID: g.NewScalar().SetUint64(1), Value: g.NewScalar().SetUint64(2)}
	shareBytes, err := marshalShare(ogShare)
	assert.NoError(t, err)
	recovShare, err := unmarshalShare(shareBytes)
	assert.NoError(t, err)
	assert.Equal(t, ogShare, recovShare)
}

func TestMarshalPointShare(t *testing.T) {
	g := group.Ristretto255
	base := g.HashToElement([]byte("base"), []byte("ss_test"))
	share := pointShare{id: g.NewScalar().SetUint64(1), point: base}
	shareBytes, err := share.marshalBinary()
	assert.NoError(t, err)
	recovShare := emptyPointShare()
	err = recovShare.unmarshalBinary(shareBytes)
	assert.NoError(t, err)
	assert.Equal(t, share.id, recovShare.id)
	sharePtBytes, err := share.point.MarshalBinary()
	assert.NoError(t, err)
	recovPtBytes, err := recovShare.point.MarshalBinary()
	assert.NoError(t, err)
	assert.Equal(t, sharePtBytes, recovPtBytes)
}
