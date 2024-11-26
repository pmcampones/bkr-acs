package asynchronousBinaryAgreement

import (
	ct "bkr-acs/coinTosser"
	on "bkr-acs/overlayNetwork"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"math/rand/v2"
	"testing"
)

func TestAbaChannelShouldDecideSelf(t *testing.T) {
	testAbaChannelShouldDecideMultiple(t, 1, 0)
}

func TestAbaChannelShouldDecideMultipleCorrect(t *testing.T) {
	testAbaChannelShouldDecideMultiple(t, 10, 0)
}

func TestAbaChannelShouldDecideMultipleWithFaults(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testAbaChannelShouldDecideMultiple(t, n, f)
}

func testAbaChannelShouldDecideMultiple(t *testing.T, n, f uint) {
	nodes := lo.Map(lo.Range(int(n)), func(i int, _ int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetTestNode(t, address, "localhost:6000")
	})
	dealSSs := lo.Map(nodes, func(node *on.Node, _ int) *on.SSChannel { return on.NewSSChannel(node, 'd') })
	ctBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.NewBEBChannel(node, 'c') })
	mBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.NewBEBChannel(node, 'm') })
	tBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.NewBEBChannel(node, 't') })
	on.InitializeNodes(t, nodes)
	assert.NoError(t, ct.DealSecret(dealSSs[0], ct.NewScalar(42), f))
	abachans := lo.ZipBy4(dealSSs, ctBebs, mBebs, tBebs, func(dealSS *on.SSChannel, ctBeb, mBeb, tBeb *on.BEBChannel) *AbaChannel {
		abachan, err := NewAbaChannel(n, f, dealSS, ctBeb, mBeb, tBeb)
		assert.NoError(t, err)
		return abachan
	})
	id := uuid.New()
	abaInstances := lo.Map(abachans, func(abachan *AbaChannel, _ int) *AbaInstance { return abachan.NewAbaInstance(id) })
	for _, instance := range abaInstances {
		assert.NoError(t, instance.Propose(byte(rand.IntN(2))))
	}
	decisions := lo.Map(abaInstances, func(instance *AbaInstance, _ int) byte { return instance.GetOutput() })
	firstDecision := decisions[0]
	assert.True(t, lo.EveryBy(decisions, func(decision byte) bool { return decision == firstDecision }))
	for _, abachan := range abachans {
		abachan.Close()
	}
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Close() == nil }))
}
