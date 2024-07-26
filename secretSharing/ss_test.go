package secretSharing

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
	base := g.HashToElement([]byte("base"), []byte("point"))
	shares := ShareSecret(threshold, nodes, secret)
	pointSecret := base.Mul(base, secret)
	hiddenShares := lo.Map(shares, func(share secretsharing.Share, _ int) PointShare { return ShareToPoint(share, base) })
	recov1 := RecoverSecretFromPoints(hiddenShares[:threshold+1])
	assert.Equal(t, pointSecret, recov1)
	recov2 := RecoverSecretFromPoints(hiddenShares[1 : threshold+2])
	fmt.Println("recov2", recov2)
	assert.Equal(t, pointSecret, recov2)
}

// TestDLEquivalence tests the equivalence of the DLEQ implementation in zk/dleq and the one in secretSharing.
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

func TestHashPointToBool(t *testing.T) {
	g := group.Ristretto255
	point := g.HashToElement([]byte("base"), []byte("point"))
	b, err := HashPointToBool(point)
	require.NoError(t, err)
	fmt.Println(b)
}
