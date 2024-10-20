package coinTosser

import (
	"bytes"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"pace/overlayNetwork"
	"testing"
)

func TestShouldDealToSelf(t *testing.T) {
	node := overlayNetwork.GetNode(t, "localhost:6000", "localhost:6000")
	ssChan, err := overlayNetwork.CreateSSChannel(node, 's')
	assert.NoError(t, err)
	overlayNetwork.InitializeNodes(t, []*overlayNetwork.Node{node})
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	err = DealSecret(ssChan, secret, 0)
	assert.NoError(t, err)
	d, err := listenDeal(ssChan.GetSSChan())
	assert.NoError(t, err)
	assert.Equal(t, secret, d.share.Value)
	assert.NoError(t, node.Disconnect())
}

func TestShouldDealToMany(t *testing.T) {
	numNodes := 10
	nodes := lo.Map(lo.Range(numNodes), func(i int, _ int) *overlayNetwork.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return overlayNetwork.GetNode(t, address, "localhost:6000")
	})
	ssChans := lo.Map(nodes, func(node *overlayNetwork.Node, _ int) *overlayNetwork.SSChannel { return makeSSChannel(t, node) })
	overlayNetwork.InitializeNodes(t, nodes)
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	err := DealSecret(ssChans[0], secret, 0)
	assert.NoError(t, err)
	deals := lo.Map(ssChans, func(ssChan *overlayNetwork.SSChannel, _ int) *deal { return listenDealTest(t, ssChan) })
	assert.True(t, lo.EveryBy(deals, func(d *deal) bool { return areScalarEquals(t, secret, d.share.Value) }))
	commitBase := deals[0].base
	assert.True(t, lo.EveryBy(deals, func(d *deal) bool { return areElementsEquals(t, commitBase, d.base) }))
	pointShares := deals[0].commits
	assert.True(t, lo.EveryBy(deals, func(d *deal) bool { return arePointShareEqualsArray(t, pointShares, d.commits) }))
	assert.True(t, lo.EveryBy(nodes, func(node *overlayNetwork.Node) bool { return node.Disconnect() == nil }))
}

func TestShouldDealRecoverableSecret(t *testing.T) {
	numNodes := 10
	nodes := lo.Map(lo.Range(numNodes), func(i int, _ int) *overlayNetwork.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return overlayNetwork.GetNode(t, address, "localhost:6000")
	})
	ssChans := lo.Map(nodes, func(node *overlayNetwork.Node, _ int) *overlayNetwork.SSChannel { return makeSSChannel(t, node) })
	overlayNetwork.InitializeNodes(t, nodes)
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	err := DealSecret(ssChans[0], secret, uint(numNodes-1))
	assert.NoError(t, err)
	deals := lo.Map(ssChans, func(ssChan *overlayNetwork.SSChannel, _ int) *deal { return listenDealTest(t, ssChan) })
	shares := lo.Map(deals, func(d *deal, _ int) secretsharing.Share { return d.share })
	recov, err := secretsharing.Recover(uint(numNodes-1), shares)
	assert.NoError(t, err)
	assert.True(t, areScalarEquals(t, secret, recov))
}

func makeSSChannel(t *testing.T, node *overlayNetwork.Node) *overlayNetwork.SSChannel {
	ssChan, err := overlayNetwork.CreateSSChannel(node, 's')
	assert.NoError(t, err)
	return ssChan
}

func listenDealTest(t *testing.T, ssChan *overlayNetwork.SSChannel) *deal {
	d, err := listenDeal(ssChan.GetSSChan())
	assert.NoError(t, err)
	return d
}

func areScalarEquals(t *testing.T, a, b group.Scalar) bool {
	aBytes, err := a.MarshalBinary()
	assert.NoError(t, err)
	bBytes, err := b.MarshalBinary()
	assert.NoError(t, err)
	return bytes.Equal(aBytes, bBytes)
}

func areElementsEquals(t *testing.T, a, b group.Element) bool {
	aBytes, err := a.MarshalBinary()
	assert.NoError(t, err)
	bBytes, err := b.MarshalBinary()
	assert.NoError(t, err)
	return bytes.Equal(aBytes, bBytes)
}

func arePointShareEqualsArray(t *testing.T, as, bs []pointShare) bool {
	return lo.EveryBy(lo.Zip2(as, bs), func(tuple lo.Tuple2[pointShare, pointShare]) bool {
		a, b := tuple.Unpack()
		return arePointShareEquals(t, a, b)
	})
}

func arePointShareEquals(t *testing.T, a, b pointShare) bool {
	return areScalarEquals(t, a.id, b.id) && areElementsEquals(t, a.point, b.point)
}
