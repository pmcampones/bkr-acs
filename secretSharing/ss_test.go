package secretSharing

import (
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v4/share"
	"testing"
)

func TestShouldMakeSS(t *testing.T) {
	nodes := 3
	threshold := 2
	secret := ToScalar(1234567890)
	shares := ShareSecret(threshold, nodes, secret)
	require.Equal(t, nodes, len(shares))
	recov1, err := RecoverSecret(shares[:threshold], threshold, nodes)
	require.NoError(t, err)
	require.Equal(t, secret, recov1)
	recov2, err := RecoverSecret(shares[1:threshold+1], threshold, nodes)
	require.NoError(t, err)
	require.Equal(t, secret, recov2)
}

func TestShouldMakeSSWithPoints(t *testing.T) {
	nodes := 3
	threshold := 2
	secret := ToScalar(1234567890)
	base := suite.Point().Pick(suite.RandomStream())
	shares := ShareSecret(threshold, nodes, secret)
	pointSecret := base.Mul(secret, base)
	hiddenShares := lo.Map(shares, func(share *share.PriShare, _ int) PointShare { return ShareToPoint(share, base) })
	recov1 := RecoverSecretFromPoints(hiddenShares[:threshold])
	require.True(t, pointSecret.Equal(recov1))
	recov2 := RecoverSecretFromPoints(hiddenShares[1 : threshold+1])
	require.True(t, pointSecret.Equal(recov2))
}
