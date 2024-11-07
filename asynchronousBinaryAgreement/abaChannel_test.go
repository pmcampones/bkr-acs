package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"math/rand/v2"
	brb "pace/byzantineReliableBroadcast"
	ct "pace/coinTosser"
	on "pace/overlayNetwork"
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
		return on.GetNode(t, address, "localhost:6000")
	})
	dealSSs := lo.Map(nodes, func(node *on.Node, _ int) *on.SSChannel { return on.CreateSSChannel(node, 'd') })
	ctBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.CreateBEBChannel(node, 'c') })
	mBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.CreateBEBChannel(node, 'm') })
	tBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.CreateBEBChannel(node, 't') })
	tBrbs := lo.Map(tBebs, func(beb *on.BEBChannel, _ int) *brb.BRBChannel { return brb.CreateBRBChannel(n, f, beb) })
	on.InitializeNodes(t, nodes)
	assert.NoError(t, ct.DealSecret(dealSSs[0], ct.NewScalar(42), f))
	abachans := lo.ZipBy4(dealSSs, ctBebs, mBebs, tBrbs, func(dealSS *on.SSChannel, ctBeb *on.BEBChannel, mBeb *on.BEBChannel, tBrb *brb.BRBChannel) *AbaChannel {
		abachan, err := NewAbaChannel(n, f, dealSS, ctBeb, mBeb, tBrb)
		assert.NoError(t, err)
		return abachan
	})
	id := uuid.New()
	resChans := lo.Map(abachans, func(abachan *AbaChannel, _ int) chan byte { return abachan.propose(id, byte(rand.IntN(2))) })
	decisions := lo.Map(resChans, func(resChan chan byte, _ int) byte { return <-resChan })
	firstDecision := decisions[0]
	assert.True(t, lo.EveryBy(decisions, func(decision byte) bool { return decision == firstDecision }))
	for _, abachan := range abachans {
		abachan.Close()
	}
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Disconnect() == nil }))
}
