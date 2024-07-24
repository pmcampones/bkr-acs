package secretSharing

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestShouldMakeSS(t *testing.T) {
	nodes := 3
	threshold := 2
	secret := ToScalar(1234567890)
	shares := ShareSecret(threshold, nodes, secret)
	require.Equal(t, nodes, len(shares))
	recov, err := RecoverSecret(shares[:threshold], threshold, nodes)
	require.NoError(t, err)
	require.Equal(t, secret, recov)
}
