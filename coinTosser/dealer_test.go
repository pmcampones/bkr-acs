package coinTosser

import (
	"github.com/cloudflare/circl/group"
	"github.com/stretchr/testify/assert"
	"pace/overlayNetwork"
	"testing"
)

func TestDealsToSelf(t *testing.T) {
	node := overlayNetwork.GetNode(t, "localhost:6000", "localhost:6000")
	ssChan, err := overlayNetwork.CreateSSChannel(node, 's')
	assert.NoError(t, err)
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	err = DealSecret(ssChan, secret, 0)
	assert.NoError(t, err)
	d, err := listenDeal(ssChan.GetSSChan())
	assert.NoError(t, err)
	assert.Equal(t, secret, d.share.Value)
}
