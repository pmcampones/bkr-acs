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
	beb := on.CreateBEBChannel(node, 'b')
	brbDeliver := make(chan BRBMsg)
	c := CreateBRBChannel(1, 0, beb, brbDeliver)
	on.InitializeNodes(t, []*on.Node{node})
	msg := []byte("hello")
	assert.NoError(t, c.BRBroadcast(msg))
	recov := <-brbDeliver
	assert.Equal(t, msg, recov.Content)
	assert.NoError(t, node.Disconnect())
	c.Close()
}

func TestChannelShouldBroadcastToAllNoFaults(t *testing.T) {
	numNodes := 10
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *on.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	channels := lo.Map(lo.Zip2(nodes, outputChans), func(t lo.Tuple2[*on.Node, chan BRBMsg], _ int) *BRBChannel {
		return getChannel(uint(numNodes), 0, t)
	})
	on.InitializeNodes(t, nodes)
	msg := []byte("hello")
	assert.NoError(t, channels[0].BRBroadcast(msg))
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(msg, recov.Content) }))
	teardown(t, channels, []*byzChannel{}, nodes)
}

func TestChannelShouldBroadcastToAllMaxFaults(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *on.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	channels := lo.Map(lo.Zip2(nodes, outputChans), func(t lo.Tuple2[*on.Node, chan BRBMsg], _ int) *BRBChannel {
		return getChannel(uint(numNodes), uint(f), t)
	})
	on.InitializeNodes(t, nodes)
	msg := []byte("hello")
	assert.NoError(t, channels[0].BRBroadcast(msg))
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(msg, recov.Content) }))
	teardown(t, channels, []*byzChannel{}, nodes)
}

func TestChannelShouldBroadcastToAllMaxCrash(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *on.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	channels := lo.Map(lo.Zip2(nodes[:numNodes-f], outputChans), func(t lo.Tuple2[*on.Node, chan BRBMsg], _ int) *BRBChannel {
		return getChannel(uint(numNodes), uint(f), t)
	})
	on.InitializeNodes(t, nodes)
	msg := []byte("hello")
	assert.NoError(t, channels[0].BRBroadcast(msg))
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(msg, recov.Content) }))
	teardown(t, channels, []*byzChannel{}, nodes)
}

func TestChannelShouldBroadcastToAllMaxByzantine(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *on.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	channels := lo.Map(lo.Zip2(nodes[:numNodes-f], outputChans), func(t lo.Tuple2[*on.Node, chan BRBMsg], _ int) *BRBChannel {
		return getChannel(uint(numNodes), uint(f), t)
	})
	byzChannels := lo.Map(nodes[numNodes-f:], func(node *on.Node, _ int) *byzChannel {
		beb := on.CreateBEBChannel(node, 'b')
		return createByzChannel(beb)
	})
	on.InitializeNodes(t, nodes)
	msg := []byte("hello")
	assert.NoError(t, channels[0].BRBroadcast(msg))
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(msg, recov.Content) }))
	teardown(t, channels, byzChannels, nodes)
}

func getNode(t *testing.T, address string) *on.Node {
	return on.GetNode(t, address, "localhost:6000")
}

func getChannel(n, f uint, tuple lo.Tuple2[*on.Node, chan BRBMsg]) *BRBChannel {
	node, brbDeliver := tuple.Unpack()
	beb := on.CreateBEBChannel(node, 'b')
	return CreateBRBChannel(n, f, beb, brbDeliver)
}

func teardown(t *testing.T, channels []*BRBChannel, byzChannels []*byzChannel, nodes []*on.Node) {
	for _, c := range channels {
		c.Close()
	}
	for _, bc := range byzChannels {
		bc.close()
	}
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Disconnect() == nil }))
}
