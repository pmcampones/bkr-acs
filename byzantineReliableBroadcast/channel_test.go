package byzantineReliableBroadcast

import (
	"bytes"
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	on "pace/overlayNetwork"
	"testing"
)

func TestChannelShouldBroadcastToSelf(t *testing.T) {
	node := getNode(t, "localhost:6000")
	beb := on.NewBEBChannel(node, 'b')
	c := NewBRBChannel(1, 0, beb)
	on.InitializeNodes(t, []*on.Node{node})
	msg := []byte("hello")
	assert.NoError(t, c.BRBroadcast(msg))
	recov := <-c.BrbDeliver
	assert.Equal(t, msg, recov.Content)
	assert.NoError(t, node.Close())
	c.Close()
}

func TestChannelShouldBroadcastToAllNoFaults(t *testing.T) {
	n := uint(10)
	testShouldBroadcastToAll(t, n, 0, n, 0)
}

func TestChannelShouldBroadcastToAllMaxFaults(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldBroadcastToAll(t, n, f, n, 0)
}

func TestChannelShouldBroadcastToAllMaxCrash(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldBroadcastToAll(t, n, f, n-f, 0)
}

func TestChannelShouldBroadcastToAllMaxByzantine(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldBroadcastToAll(t, n, f, n-f, f)
}

func testShouldBroadcastToAll(t *testing.T, n, f, correct, byzantine uint) {
	addresses := lo.Map(lo.Range(int(n)), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *on.Node { return getNode(t, address) })
	channels := lo.Map(nodes[:correct], func(node *on.Node, _ int) *BRBChannel {
		return getChannel(n, f, node)
	})
	byzChannels := lo.Map(nodes[correct:correct+byzantine], func(node *on.Node, _ int) *byzChannel {
		beb := on.NewBEBChannel(node, 'b')
		return createByzChannel(beb)
	})
	on.InitializeNodes(t, nodes)
	msg := []byte("hello")
	assert.NoError(t, channels[0].BRBroadcast(msg))
	outputs := lo.Map(channels, func(c *BRBChannel, _ int) BRBMsg { return <-c.BrbDeliver })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(msg, recov.Content) }))
	teardown(t, channels, byzChannels, nodes)
}

func getNode(t *testing.T, address string) *on.Node {
	return on.GetTestNode(t, address, "localhost:6000")
}

func getChannel(n, f uint, node *on.Node) *BRBChannel {
	beb := on.NewBEBChannel(node, 'b')
	return NewBRBChannel(n, f, beb)
}

func teardown(t *testing.T, channels []*BRBChannel, byzChannels []*byzChannel, nodes []*on.Node) {
	for _, c := range channels {
		c.Close()
	}
	for _, bc := range byzChannels {
		bc.close()
	}
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Close() == nil }))
}
