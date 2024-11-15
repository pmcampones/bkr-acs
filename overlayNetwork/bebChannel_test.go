package overlayNetwork

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type nodeMsg struct {
	beb   *BEBChannel
	bMsgs [][]byte // messages to be broadcast by the node
	rMsgs [][]byte // messages to be received by the node
}

func newNodeMsg(beb *BEBChannel, bMsgs [][]byte) *nodeMsg {
	return &nodeMsg{beb, bMsgs, make([][]byte, 0)}
}

func TestShouldBroadcastSelf(t *testing.T) {
	contact := "localhost:6000"
	node := getNode(t, contact)
	InitializeNodes(t, []*Node{node})
	beb := NewBEBChannel(node, 'b')
	nmsg := newNodeMsg(beb, [][]byte{[]byte("hello")})
	testShouldBroadcast(t, []*nodeMsg{nmsg})
	assert.Equal(t, 1, len(nmsg.rMsgs))
	assert.Equal(t, "hello", string(nmsg.rMsgs[0]))
	assert.NoError(t, node.Close())
}

func TestShouldBroadcastSelfManyMessages(t *testing.T) {
	contact := "localhost:6000"
	node := getNode(t, contact)
	beb := NewBEBChannel(node, 'b')
	InitializeNodes(t, []*Node{node})
	numMsgs := 10000
	msgs := genNodeMsgs(0, numMsgs)
	nmsg := newNodeMsg(beb, msgs)
	testShouldBroadcast(t, []*nodeMsg{nmsg})
	assert.Equal(t, numMsgs, len(nmsg.rMsgs))
	assert.NoError(t, node.Close())
}

func TestShouldBroadcastTwoNodesSingleMessage(t *testing.T) {
	contact := "localhost:6000"
	address1 := "localhost:6001"
	node0 := getNode(t, contact)
	node1 := getNode(t, address1)
	beb0 := NewBEBChannel(node0, 'b')
	beb1 := NewBEBChannel(node1, 'b')
	InitializeNodes(t, []*Node{node0, node1})
	nmsg0 := newNodeMsg(beb0, [][]byte{[]byte("hello")})
	nmsg1 := newNodeMsg(beb1, [][]byte{})
	testShouldBroadcast(t, []*nodeMsg{nmsg0, nmsg1})
	assert.Equal(t, 1, len(nmsg0.rMsgs))
	assert.Equal(t, 1, len(nmsg1.rMsgs))
	assert.Equal(t, "hello", string(nmsg0.rMsgs[0]))
	assert.Equal(t, "hello", string(nmsg1.rMsgs[0]))
	assert.NoError(t, node0.Close())
	assert.NoError(t, node1.Close())
}

func TestShouldBroadcastTwoNodesManyMessages(t *testing.T) {
	contact := "localhost:6000"
	address1 := "localhost:6001"
	node0 := getNode(t, contact)
	node1 := getNode(t, address1)
	beb0 := NewBEBChannel(node0, 'b')
	beb1 := NewBEBChannel(node1, 'b')
	InitializeNodes(t, []*Node{node0, node1})
	numMsgs := 10000
	msgs0 := genNodeMsgs(0, numMsgs)
	msgs1 := genNodeMsgs(1, numMsgs)
	nmsg0 := newNodeMsg(beb0, msgs0)
	nmsg1 := newNodeMsg(beb1, msgs1)
	testShouldBroadcast(t, []*nodeMsg{nmsg0, nmsg1})
	assert.Equal(t, len(msgs0)+len(msgs1), len(nmsg0.rMsgs))
	assert.Equal(t, len(msgs0)+len(msgs1), len(nmsg1.rMsgs))
	assert.NoError(t, node0.Close())
	assert.NoError(t, node1.Close())
}

func TestShouldBroadcastManyNodesManyMessages(t *testing.T) {
	numNodes := 100
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *Node { return getNode(t, address) })
	bebs := lo.Map(nodes, func(n *Node, _ int) *BEBChannel { return NewBEBChannel(n, 'b') })
	InitializeNodes(t, nodes)
	numMsgs := 100
	msgs := lo.Map(lo.Range(numNodes), func(_ int, i int) [][]byte { return genNodeMsgs(i, numMsgs) })
	nodeMsgs := lo.ZipBy2(bebs, msgs, func(b *BEBChannel, msgs [][]byte) *nodeMsg { return newNodeMsg(b, msgs) })
	testShouldBroadcast(t, nodeMsgs)
	totalMsgs := numMsgs * numNodes
	assert.True(t, lo.EveryBy(nodeMsgs, func(nm *nodeMsg) bool { return len(nm.rMsgs) == totalMsgs }))
	assert.True(t, lo.EveryBy(nodes, func(n *Node) bool { return n.Close() == nil }))
}

func genNodeMsgs(nodeIdx, numMsgs int) [][]byte {
	return lo.Map(lo.Range(numMsgs), func(_ int, i int) []byte { return []byte(fmt.Sprintf("hello %d %d", nodeIdx, i)) })
}

func testShouldBroadcast(t *testing.T, nodeMsgs []*nodeMsg) {
	totalMsgs := lo.Sum(lo.Map(nodeMsgs, func(nm *nodeMsg, _ int) int { return len(nm.bMsgs) }))
	broadcastAllMsgs(t, nodeMsgs)
	for _, nm := range nodeMsgs {
		for i := 0; i < totalMsgs; i++ {
			msg := <-nm.beb.deliverChan
			nm.rMsgs = append(nm.rMsgs, msg.Content)
		}
	}
}

func broadcastAllMsgs(t *testing.T, nodeMsgs []*nodeMsg) {
	for _, nm := range nodeMsgs {
		for _, msg := range nm.bMsgs {
			require.NoError(t, nm.beb.BEBroadcast(msg))
		}
	}
}

func getNode(t *testing.T, address string) *Node {
	return GetTestNode(t, address, "localhost:6000")
}
