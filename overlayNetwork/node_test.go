package overlayNetwork

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type dummyMsgObserver struct {
	msgs chan []byte
}

func newDummyMsgObserver() *dummyMsgObserver {
	return &dummyMsgObserver{
		msgs: make(chan []byte),
	}
}

func (dmo *dummyMsgObserver) BEBDeliver(msg []byte, _ *ecdsa.PublicKey) {
	dmo.msgs <- msg
}

type nodeMsg struct {
	node   *Node
	msgObs *dummyMsgObserver
	bMsgs  [][]byte // messages to be broadcast by the node
	rMsgs  [][]byte // messages to be received by the node
}

func newNodeMsg(node *Node, bMsgs [][]byte) *nodeMsg {
	msgObs := newDummyMsgObserver()
	node.AttachMessageObserver(msgObs)
	return &nodeMsg{node, msgObs, bMsgs, make([][]byte, 0)}
}

func TestShouldBroadcastSelf(t *testing.T) {
	contact := "localhost:6000"
	node := getNode(t, contact)
	nmsg := newNodeMsg(node, [][]byte{[]byte("hello")})
	testShouldBroadcast(t, []*nodeMsg{nmsg}, contact)
	assert.Equal(t, 1, len(nmsg.rMsgs))
	assert.Equal(t, "hello", string(nmsg.rMsgs[0]))
	assert.NoError(t, node.Disconnect())
}

func TestShouldBroadcastSelfManyMessages(t *testing.T) {
	contact := "localhost:6000"
	node := getNode(t, contact)
	numMsgs := 10000
	msgs := genNodeMsgs(0, numMsgs)
	nmsg := newNodeMsg(node, msgs)
	testShouldBroadcast(t, []*nodeMsg{nmsg}, contact)
	assert.Equal(t, numMsgs, len(nmsg.rMsgs))
	assert.NoError(t, node.Disconnect())
}

func TestShouldBroadcastTwoNodesSingleMessage(t *testing.T) {
	contact := "localhost:6000"
	address1 := "localhost:6001"
	node0 := getNode(t, contact)
	node1 := getNode(t, address1)
	nmsg0 := newNodeMsg(node0, [][]byte{[]byte("hello")})
	nmsg1 := newNodeMsg(node1, [][]byte{})
	testShouldBroadcast(t, []*nodeMsg{nmsg0, nmsg1}, contact)
	assert.Equal(t, 1, len(nmsg0.rMsgs))
	assert.Equal(t, 1, len(nmsg1.rMsgs))
	assert.Equal(t, "hello", string(nmsg0.rMsgs[0]))
	assert.Equal(t, "hello", string(nmsg1.rMsgs[0]))
	assert.NoError(t, node0.Disconnect())
	assert.NoError(t, node1.Disconnect())
}

func TestShouldBroadcastTwoNodesManyMessages(t *testing.T) {
	contact := "localhost:6000"
	address1 := "localhost:6001"
	node0 := getNode(t, contact)
	node1 := getNode(t, address1)
	numMsgs := 10000
	msgs0 := genNodeMsgs(0, numMsgs)
	msgs1 := genNodeMsgs(1, numMsgs)
	nmsg0 := newNodeMsg(node0, msgs0)
	nmsg1 := newNodeMsg(node1, msgs1)
	testShouldBroadcast(t, []*nodeMsg{nmsg0, nmsg1}, contact)
	assert.Equal(t, len(msgs0)+len(msgs1), len(nmsg0.rMsgs))
	assert.Equal(t, len(msgs0)+len(msgs1), len(nmsg1.rMsgs))
	assert.NoError(t, node0.Disconnect())
	assert.NoError(t, node1.Disconnect())
}

func TestShouldBroadcastManyNodesManyMessages(t *testing.T) {
	contact := "localhost:6000"
	numNodes := 100
	addresses := lo.Map(lo.Range(numNodes), func(_ int, i int) string { return fmt.Sprintf("localhost:%d", 6000+i) })
	nodes := lo.Map(addresses, func(address string, _ int) *Node { return getNode(t, address) })
	numMsgs := 100
	msgs := lo.Map(lo.Range(numNodes), func(_ int, i int) [][]byte { return genNodeMsgs(i, numMsgs) })
	nodeMsgs := lo.ZipBy2(nodes, msgs, func(node *Node, msgs [][]byte) *nodeMsg { return newNodeMsg(node, msgs) })
	testShouldBroadcast(t, nodeMsgs, contact)
	totalMsgs := numMsgs * numNodes
	assert.True(t, lo.EveryBy(nodeMsgs, func(nm *nodeMsg) bool { return len(nm.rMsgs) == totalMsgs }))
	assert.True(t, lo.EveryBy(nodeMsgs, func(nm *nodeMsg) bool { return nm.node.Disconnect() == nil }))
}

func genNodeMsgs(nodeIdx, numMsgs int) [][]byte {
	return lo.Map(lo.Range(numMsgs), func(_ int, i int) []byte { return []byte(fmt.Sprintf("hello %d %d", nodeIdx, i)) })
}

func testShouldBroadcast(t *testing.T, nodeMsgs []*nodeMsg, contact string) {
	nodes := lo.Map(nodeMsgs, func(nm *nodeMsg, _ int) *Node { return nm.node })
	InitializeNodes(t, nodes, contact)
	totalMsgs := lo.Sum(lo.Map(nodeMsgs, func(nm *nodeMsg, _ int) int { return len(nm.bMsgs) }))
	broadcastAllMsgs(t, nodeMsgs)
	for _, nm := range nodeMsgs {
		for i := 0; i < totalMsgs; i++ {
			nm.rMsgs = append(nm.rMsgs, <-nm.msgObs.msgs)
		}
	}
}

func broadcastAllMsgs(t *testing.T, nodeMsgs []*nodeMsg) {
	for _, nm := range nodeMsgs {
		for _, msg := range nm.bMsgs {
			err := nm.node.Broadcast(msg)
			require.NoError(t, err)
		}
	}
}

func getNode(t *testing.T, address string) *Node {
	return GetNode(t, address, "localhost:6000")
}
