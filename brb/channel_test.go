package brb

import (
	"bytes"
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"pace/overlayNetwork"
	"testing"
)

func TestChannelShouldBroadcastToSelf(t *testing.T) {
	node := getNode(t, "localhost:6000")
	brbDeliver := make(chan []byte)
	c := CreateBRBChannel(1, 0, node, brbDeliver, 'B')
	overlayNetwork.InitializeNodes(t, []*overlayNetwork.Node{node}, "localhost:6000")
	msg := []byte("hello")
	err := c.BRBroadcast(msg)
	assert.NoError(t, err)
	recov := <-brbDeliver
	assert.Equal(t, recov, msg)
	assert.NoError(t, node.Disconnect())
	c.Close()
}

func TestChannelShouldBroadcastToAllNoFaults(t *testing.T) {
	numNodes := 10
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *overlayNetwork.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	channels := lo.Map(lo.Zip2(nodes, outputChans), func(t lo.Tuple2[*overlayNetwork.Node, chan []byte], _ int) *BRBChannel {
		return getChannel(uint(numNodes), 0, t)
	})
	overlayNetwork.InitializeNodes(t, nodes, "localhost:6000")
	msg := []byte("hello")
	err := channels[0].BRBroadcast(msg)
	assert.NoError(t, err)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(msg, recov) }))
	teardown(t, channels, []*byzChannel{}, nodes)
}

func TestChannelShouldBroadcastToAllMaxFaults(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *overlayNetwork.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	channels := lo.Map(lo.Zip2(nodes, outputChans), func(t lo.Tuple2[*overlayNetwork.Node, chan []byte], _ int) *BRBChannel {
		return getChannel(uint(numNodes), uint(f), t)
	})
	overlayNetwork.InitializeNodes(t, nodes, "localhost:6000")
	msg := []byte("hello")
	err := channels[0].BRBroadcast(msg)
	assert.NoError(t, err)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(msg, recov) }))
	teardown(t, channels, []*byzChannel{}, nodes)
}

func TestChannelShouldBroadcastToAllMaxCrash(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *overlayNetwork.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	channels := lo.Map(lo.Zip2(nodes[:numNodes-f], outputChans), func(t lo.Tuple2[*overlayNetwork.Node, chan []byte], _ int) *BRBChannel {
		return getChannel(uint(numNodes), uint(f), t)
	})
	overlayNetwork.InitializeNodes(t, nodes, "localhost:6000")
	msg := []byte("hello")
	err := channels[0].BRBroadcast(msg)
	assert.NoError(t, err)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(msg, recov) }))
	teardown(t, channels, []*byzChannel{}, nodes)
}

func TestChannelShouldBroadcastToAllMaxByzantine(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *overlayNetwork.Node { return getNode(t, address) })
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	channels := lo.Map(lo.Zip2(nodes[:numNodes-f], outputChans), func(t lo.Tuple2[*overlayNetwork.Node, chan []byte], _ int) *BRBChannel {
		return getChannel(uint(numNodes), uint(f), t)
	})
	byzChannels := lo.Map(nodes[numNodes-f:], func(node *overlayNetwork.Node, _ int) *byzChannel {
		return createByzChannel(node, 'B')
	})
	overlayNetwork.InitializeNodes(t, nodes, "localhost:6000")
	msg := []byte("hello")
	err := channels[0].BRBroadcast(msg)
	assert.NoError(t, err)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(msg, recov) }))
	teardown(t, channels, byzChannels, nodes)
}

func getNode(t *testing.T, address string) *overlayNetwork.Node {
	return overlayNetwork.GetNode(t, address, "localhost:6000")
}

func getChannel(n, f uint, tuple lo.Tuple2[*overlayNetwork.Node, chan []byte]) *BRBChannel {
	node, brbDeliver := tuple.Unpack()
	return CreateBRBChannel(n, f, node, brbDeliver, 'B')
}

func teardown(t *testing.T, channels []*BRBChannel, byzChannels []*byzChannel, nodes []*overlayNetwork.Node) {
	for _, c := range channels {
		c.Close()
	}
	for _, bc := range byzChannels {
		bc.close()
	}
	assert.True(t, lo.EveryBy(nodes, func(node *overlayNetwork.Node) bool { return node.Disconnect() == nil }))
}
