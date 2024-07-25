package secretSharing

import (
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
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
