package coinTosser

import (
	"fmt"
	"github.com/cloudflare/circl/group"
	ss "github.com/cloudflare/circl/secretsharing"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	on "pace/overlayNetwork"
	"testing"
)

func TestShouldDealToSelf(t *testing.T) {
	node := on.GetNode(t, "localhost:6000", "localhost:6000")
	ssChan := on.CreateSSChannel(node, 's')
	on.InitializeNodes(t, []*on.Node{node})
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	err := DealSecret(ssChan, secret, 0)
	assert.NoError(t, err)
	d, err := listenDeal(ssChan.GetSSChan())
	assert.NoError(t, err)
	assert.Equal(t, secret, d.share.Value)
	assert.NoError(t, node.Disconnect())
}

func TestShouldDealToMany(t *testing.T) {
	numNodes := 10
	nodes := lo.Map(lo.Range(numNodes), func(i int, _ int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetNode(t, address, "localhost:6000")
	})
	ssChans := lo.Map(nodes, func(node *on.Node, _ int) *on.SSChannel { return on.CreateSSChannel(node, 's') })
	on.InitializeNodes(t, nodes)
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	err := DealSecret(ssChans[0], secret, 0)
	assert.NoError(t, err)
	deals := lo.Map(ssChans, func(ssChan *on.SSChannel, _ int) *deal { return listenDealTest(t, ssChan) })
	assert.True(t, lo.EveryBy(deals, func(d *deal) bool { return areScalarEqualsTest(t, secret, d.share.Value) }))
	commitBase := deals[0].base
	assert.True(t, lo.EveryBy(deals, func(d *deal) bool { return areElementsEqualsTest(t, commitBase, d.base) }))
	pointShares := deals[0].commits
	assert.True(t, lo.EveryBy(deals, func(d *deal) bool { return arePointShareEqualsArray(t, pointShares, d.commits) }))
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Disconnect() == nil }))
}

func TestShouldDealRecoverableSecret(t *testing.T) {
	numNodes := 10
	nodes := lo.Map(lo.Range(numNodes), func(i int, _ int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetNode(t, address, "localhost:6000")
	})
	ssChans := lo.Map(nodes, func(node *on.Node, _ int) *on.SSChannel { return on.CreateSSChannel(node, 's') })
	on.InitializeNodes(t, nodes)
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	err := DealSecret(ssChans[0], secret, uint(numNodes-1))
	assert.NoError(t, err)
	deals := lo.Map(ssChans, func(ssChan *on.SSChannel, _ int) *deal { return listenDealTest(t, ssChan) })
	shares := lo.Map(deals, func(d *deal, _ int) ss.Share { return d.share })
	recov, err := ss.Recover(uint(numNodes-1), shares)
	assert.NoError(t, err)
	assert.True(t, areScalarEqualsTest(t, secret, recov))
}

func listenDealTest(t *testing.T, ssChan *on.SSChannel) *deal {
	d, err := listenDeal(ssChan.GetSSChan())
	assert.NoError(t, err)
	return d
}

func areScalarEqualsTest(t *testing.T, a, b group.Scalar) bool {
	ok, err := areScalarEquals(a, b)
	assert.NoError(t, err)
	return ok
}

func areElementsEqualsTest(t *testing.T, a, b group.Element) bool {
	ok, err := areElementsEquals(a, b)
	assert.NoError(t, err)
	return ok
}

func arePointShareEqualsArray(t *testing.T, as, bs []pointShare) bool {
	return lo.EveryBy(lo.Zip2(as, bs), func(tuple lo.Tuple2[pointShare, pointShare]) bool {
		a, b := tuple.Unpack()
		return arePointShareEquals(t, a, b)
	})
}

func arePointShareEquals(t *testing.T, a, b pointShare) bool {
	return areScalarEqualsTest(t, a.id, b.id) && areElementsEqualsTest(t, a.point, b.point)
}
