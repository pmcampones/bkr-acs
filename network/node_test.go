package network

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
	"time"
)

func TestShouldBroadcastSingleNode(t *testing.T) {
	contact := "localhost:6000"
	node, _, msgObs, err := MakeNode(contact, contact, 1, 1)
	if err != nil {
		t.Fatalf("unable to create node sender: %v", err)
	}
	msg := []byte("hello")
	assert.Equal(t, len(msgObs.delivered), 0)
	node.Broadcast(msg)
	<-msgObs.barrier
	assert.Equal(t, msgObs.delivered[string(msg)], true)
	assert.Equal(t, len(msgObs.delivered), 1)
}

func TestShouldBroadcastSingleNodeManyMessages(t *testing.T) {
	contact := "localhost:6000"
	numMsgs := 10000
	msgs := make([][]byte, numMsgs)
	for i := 0; i < numMsgs; i++ {
		msgs[i] = []byte(fmt.Sprintf("hello %d", i))
	}
	node, _, msgObs, err := MakeNode(contact, contact, numMsgs, 1)
	if err != nil {
		t.Fatalf("unable to create node sender: %v", err)
	}
	for _, msg := range msgs {
		node.Broadcast(msg)
	}
	for i := 0; i < numMsgs; i++ {
		<-msgObs.barrier
	}
	for _, msg := range msgs {
		assert.Equal(t, msgObs.delivered[string(msg)], true)
	}
	assert.Equal(t, len(msgObs.delivered), len(msgs))
}

func TestShouldBroadcastTwoNodesSingleMessage(t *testing.T) {
	address1 := "localhost:6000"
	address2 := "localhost:6001"
	node1, memObs1, msgObs1, err := MakeNode(address1, address1, 2, 1)
	if err != nil {
		t.Fatalf("unable to create node address1: %v", err)
	}
	node2, memObs2, msgObs2, err := MakeNode(address2, address1, 2, 1)
	if err != nil {
		t.Fatalf("unable to create node address2: %v", err)
	}
	<-memObs1.UpBarrier
	<-memObs2.UpBarrier
	node1.Broadcast([]byte(address1))
	<-msgObs1.barrier
	<-msgObs2.barrier
	assert.Equal(t, len(msgObs1.delivered), 1)
	assert.Equal(t, len(msgObs2.delivered), 1)
	assert.Equal(t, msgObs1.delivered[address1], true)
	assert.Equal(t, msgObs2.delivered[address1], true)
	node2.Broadcast([]byte(address2))
	<-msgObs1.barrier
	<-msgObs2.barrier
	assert.Equal(t, len(msgObs1.delivered), 2)
	assert.Equal(t, len(msgObs2.delivered), 2)
	assert.Equal(t, msgObs1.delivered[address2], true)
	assert.Equal(t, msgObs2.delivered[address2], true)
}

func TestShouldBroadcastTwoNodesManyMessages(t *testing.T) {
	address1 := "localhost:6000"
	address2 := "localhost:6001"
	numMsgs := 10000
	msgs1 := make([][]byte, numMsgs)
	msgs2 := make([][]byte, numMsgs)
	for i := 0; i < numMsgs; i++ {
		msgs1[i] = []byte(fmt.Sprintf("hello %d", i))
		msgs2[i] = []byte(fmt.Sprintf("hello %d", i+numMsgs))
	}
	node1, memObs1, msgObs1, err := MakeNode(address1, address1, 2*numMsgs, 1)
	if err != nil {
		t.Fatalf("unable to create node address1: %v", err)
	}
	node2, memObs2, msgObs2, err := MakeNode(address2, address1, 2*numMsgs, 1)
	if err != nil {
		t.Fatalf("unable to create node address2: %v", err)
	}
	<-memObs1.UpBarrier
	<-memObs2.UpBarrier
	for i := 0; i < numMsgs; i++ {
		node1.Broadcast(msgs1[i])
		node2.Broadcast(msgs2[i])
	}
	for i := 0; i < numMsgs; i++ {
		<-msgObs1.barrier
		<-msgObs1.barrier
		<-msgObs2.barrier
		<-msgObs2.barrier
	}
	for i := 0; i < numMsgs; i++ {
		assert.Equal(t, msgObs1.delivered[string(msgs1[i])], true)
		assert.Equal(t, msgObs1.delivered[string(msgs2[i])], true)
		assert.Equal(t, msgObs2.delivered[string(msgs1[i])], true)
		assert.Equal(t, msgObs2.delivered[string(msgs2[i])], true)
	}
	assert.Equal(t, len(msgObs1.delivered), 2*numMsgs)
	assert.Equal(t, len(msgObs2.delivered), 2*numMsgs)
}

func TestShouldBroadcastManyNodesManyMessages(t *testing.T) {
	contact := "localhost:6000"
	numNodes := 100
	numMsgs := 100
	nodes := make([]*Node, numNodes)
	memObs := make([]*TestMemObserver, numNodes)
	msgObs := make([]*TestMsgObserver, numNodes)
	msgs := make([][][]byte, numNodes)
	for i := 0; i < numNodes; i++ {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		nodes[i], memObs[i], msgObs[i], _ = MakeNode(address, contact, numMsgs*numNodes, numNodes)
		msgs[i] = make([][]byte, numMsgs)
		for j := 0; j < numMsgs; j++ {
			msgs[i][j] = []byte(fmt.Sprintf("hello %d %d", i, j))
		}
		for j := 1; j < i; j++ {
			<-memObs[i].UpBarrier
		}
	}
	fmt.Println("Generated nodes and messages")
	for i, node := range nodes {
		for _, msg := range msgs[i] {
			node.Broadcast(msg)
		}
	}
	fmt.Println("Messages networkChannel")
	for _, msgOb := range msgObs {
		for j := 0; j < numMsgs*numNodes; j++ {
			<-msgOb.barrier
		}
	}
	fmt.Println("Messages received")
	for _, msgOb := range msgObs {
		assert.Equal(t, len(msgOb.delivered), numMsgs*numNodes)
		for _, nodeMsgs := range msgs {
			for _, msg := range nodeMsgs {
				assert.Equal(t, msgOb.delivered[string(msg)], true)
			}
		}
	}
}

func TestShouldUnicastSingleMessage(t *testing.T) {
	contact := "localhost:6000"
	peer1Name := "localhost:6001"
	peer2Name := "localhost:6002"
	node0, memObs, msgObs0, err := MakeNode(contact, contact, 10, 10)
	if err != nil {
		t.Fatalf("unable to create node sender: %v", err)
	}
	_, _, msgObs1, err := MakeNode(peer1Name, contact, 10, 10)
	if err != nil {
		t.Fatalf("unable to create node receiver: %v", err)
	}
	_, _, msgObs2, err := MakeNode(peer2Name, contact, 10, 10)
	if err != nil {
		t.Fatalf("unable to create node receiver: %v", err)
	}
	msg := []byte("hello")
	<-memObs.UpBarrier
	<-memObs.UpBarrier
	peer1 := memObs.Peers[peer1Name]
	if peer1 == nil {
		t.Fatalf("unable to find peer1: %v", peer1Name)
	}
	node0.Unicast(msg, peer1.Conn)
	time.Sleep(1 * time.Second)
	assert.Equal(t, len(msgObs0.delivered), 0)
	assert.Equal(t, len(msgObs1.delivered), 1)
	assert.Equal(t, len(msgObs2.delivered), 0)
	assert.Equal(t, msgObs1.delivered[string(msg)], true)
}
