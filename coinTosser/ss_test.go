package coinTosser

import (
	"crypto"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"github.com/cloudflare/circl/zk/dleq"
	"github.com/magiconair/properties/assert"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRecoverSecret(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(50)
	threshold := uint(20)
	secret := g.NewScalar().SetUint64(1234567890)
	shares := ShareSecret(threshold, nodes, secret)
	require.Equal(t, int(nodes), len(shares))
	recov1, err := RecoverSecret(threshold, shares[:threshold+1])
	require.NoError(t, err)
	assert.Equal(t, secret, recov1)
	recov2, err := RecoverSecret(threshold, shares[1:threshold+2])
	require.NoError(t, err)
	assert.Equal(t, secret, recov2)
}

func TestSSWithPoints(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(50)
	threshold := uint(20)
	secret := g.NewScalar().SetUint64(1234567890)
	base := g.HashToElement([]byte("base"), []byte("ss_tests"))
	shares := ShareSecret(threshold, nodes, secret)
	pointSecret := g.Identity().Mul(base, secret)
	hiddenShares := lo.Map(shares, func(share secretsharing.Share, _ int) pointShare { return ShareToPoint(share, base) })
	recov1 := RecoverSecretFromPoints(hiddenShares[:])
	ptSecBytes, err := pointSecret.MarshalBinary()
	require.NoError(t, err)
	recSecBytes, err := recov1.MarshalBinary()
	require.NoError(t, err)
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

	fmt.Printf("Value to prove: %v\n", x)
	fmt.Printf("Public value (A):\n%v\n", A)
	fmt.Printf("Public value (B):\n%v\n", B)
	fmt.Printf("Domain seperation: %s\n\n", dst)

	rr := g.HashToScalar([]byte("randomness"), []byte("randomness"))
	proof, _ := prover.ProveWithRandomness(x, G, A, H, B, rr)
	p, _ := proof.MarshalBinary()
	fmt.Printf("\nProof with randomness: %+v\n", hex.EncodeToString(p))
	rtn := verifier.Verify(G, A, H, B, proof)

	fmt.Printf("Verification: %+v\n", rtn)
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
	shares := ShareSecret(threshold, nodes, secret)
	rnd := g.HashToScalar([]byte("randomVal"), []byte("1"))
	for _, share := range shares {
		fmt.Println("Share idx:", share.ID)
		hiddenShare := g.NewElement().Mul(randomBase, share.Value)
		commitment := g.NewElement().Mul(commitBase, share.Value)
		proof, err := prover.ProveWithRandomness(share.Value, randomBase, hiddenShare, commitBase, commitment, rnd)
		require.NoError(t, err)
		res := verifier.Verify(randomBase, hiddenShare, commitBase, commitment, proof)
		assert.Equal(t, res, true)
	}
}

func TestHashPointToBool(t *testing.T) {
	g := group.Ristretto255
	point := g.HashToElement([]byte("base"), []byte("point"))
	b, err := HashPointToBool(point)
	require.NoError(t, err)
	fmt.Println(b)
}

func TestMarshalAndUnmarshal(t *testing.T) {
	g := group.Ristretto255
	ogShare := secretsharing.Share{ID: g.NewScalar().SetUint64(1), Value: g.NewScalar().SetUint64(2)}
	shareBytes, err := marshalShare(ogShare)
	require.NoError(t, err)
	recovShare, err := unmarshalShare(shareBytes)
	require.NoError(t, err)
	assert.Equal(t, ogShare, recovShare)
}

func TestMarshalPointShare(t *testing.T) {
	g := group.Ristretto255
	base := g.HashToElement([]byte("base"), []byte("point"))
	share := pointShare{id: g.NewScalar().SetUint64(1), point: base}
	shareBytes, err := marshalPointShare(share)
	require.NoError(t, err)
	recovShare, err := unmarshalPointShare(shareBytes)
	require.NoError(t, err)
	assert.Equal(t, share.id, recovShare.id)
	sharePtBytes, err := share.point.MarshalBinary()
	require.NoError(t, err)
	recovPtBytes, err := recovShare.point.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, sharePtBytes, recovPtBytes)
}
