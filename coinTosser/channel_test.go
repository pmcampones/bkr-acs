package coinTosser

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"pace/overlayNetwork"
	"testing"
)

func TestChannelShouldDeliverOwnCoin(t *testing.T) {
	testShouldDeliverWithTheshold(t, 1, 0)
}

func TestChannelShouldDeliverManyNodesNoThreshold(t *testing.T) {
	testShouldDeliverWithTheshold(t, 10, 0)
}

func TestChannelShouldDeliverManyNodesHalfThreshold(t *testing.T) {
	numNodes := uint(10)
	threshold := numNodes / 2
	testShouldDeliverWithTheshold(t, numNodes, threshold)
}

func TestChannelShouldDeliverManyNodesFullThreshold(t *testing.T) {
	numNodes := uint(10)
	threshold := numNodes - 1
	testShouldDeliverWithTheshold(t, numNodes, threshold)
}

func testShouldDeliverWithTheshold(t *testing.T, numNodes, threshold uint) {
	nodes := lo.Map(lo.Range(int(numNodes)), func(i int, _ int) *overlayNetwork.Node {
		return overlayNetwork.GetNode(t, fmt.Sprintf("localhost:%d", 6000+i), "localhost:6000")
	})
	ssChans := lo.Map(nodes, func(n *overlayNetwork.Node, _ int) *overlayNetwork.SSChannel { return makeSSChannel(t, n) })
	bebChans := lo.Map(nodes, func(n *overlayNetwork.Node, _ int) *overlayNetwork.BEBChannel {
		return overlayNetwork.CreateBEBChannel(n, 'c')
	})
	overlayNetwork.InitializeNodes(t, nodes)
	ctChannels := lo.ZipBy2(ssChans, bebChans, func(ss *overlayNetwork.SSChannel, beb *overlayNetwork.BEBChannel) *CTChannel {
		ct, err := NewCoinTosserChannel(ss, beb, threshold)
		assert.NoError(t, err)
		return ct
	})
	secret := newScalar(42)
	err := DealSecret(ssChans[0], secret, threshold)
	assert.NoError(t, err)
	outputChans := lo.Map(ctChannels, func(ct *CTChannel, _ int) chan bool { return make(chan bool) })
	for _, tuple := range lo.Zip2(ctChannels, outputChans) {
		ct, oc := tuple.Unpack()
		ct.TossCoin([]byte("test"), oc)
	}
	outcomes := lo.Map(outputChans, func(oc chan bool, _ int) bool { return <-oc })
	firstOutcome := outcomes[0]
	assert.True(t, lo.EveryBy(outcomes, func(outcome bool) bool { return outcome == firstOutcome }))
	for _, ct := range ctChannels {
		ct.Close()
	}
	assert.True(t, lo.EveryBy(nodes, func(n *overlayNetwork.Node) bool { return n.Disconnect() == nil }))
}
