package agreementCommonSubset

import (
	aba "bkr-acs/asynchronousBinaryAgreement"
	brb "bkr-acs/byzantineReliableBroadcast"
	on "bkr-acs/overlayNetwork"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

func TestChannelShouldAcceptOwnProposal(t *testing.T) {
	node := on.GetTestNode(t, "localhost:6000", "localhost:6000")
	proposer, err := node.GetId()
	assert.NoError(t, err)
	bebChan := on.NewBEBChannel(node, 'z')
	brbChan := brb.NewBRBChannel(1, 0, bebChan)
	abaChan := getAbachans(t, 1, 0, []*on.Node{node})[0]
	bkrChan := NewBKRChannel(0, abaChan, brbChan, []uuid.UUID{proposer})
	id := uuid.New()
	getListener := make(chan chan [][]byte, 1)
	go func() {
		outputListener, err := bkrChan.Propose(id, []byte("Hello World"))
		assert.NoError(t, err)
		getListener <- outputListener
	}()
	outputListener := <-getListener
	res := <-outputListener
	assert.Equal(t, 1, len(res))
	assert.True(t, slices.Equal([]byte("Hello World"), res[0]))
	assert.NoError(t, node.Close())
}

func TestChannelShouldAgreeProposalsNoFaults(t *testing.T) {
	testChannelShouldAgreeProposals(t, 10, 0, 300)
}

func TestChannelShouldAgreeProposalMaxFaults(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testChannelShouldAgreeProposals(t, n, f, 300)
}

func testChannelShouldAgreeProposals(t *testing.T, n, f uint, maxDelay uint) {
	nodes := lo.Map(lo.Range(int(n)), func(_ int, i int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetTestNode(t, address, "localhost:6000")
	})
	proposers := lo.Map(nodes, func(n *on.Node, _ int) uuid.UUID {
		id, err := n.GetId()
		assert.NoError(t, err)
		return id
	})
	bebChans := lo.Map(nodes, func(n *on.Node, _ int) *on.BEBChannel {
		return on.NewBEBChannel(n, 'z')
	})
	brbChans := lo.Map(bebChans, func(b *on.BEBChannel, _ int) *brb.BRBChannel {
		return brb.NewBRBChannel(n, f, b)
	})
	abaChans := getAbachans(t, n, f, nodes)
	bkrChans := lo.ZipBy2(abaChans, brbChans, func(a *aba.AbaChannel, b *brb.BRBChannel) *BKRChannel {
		return NewBKRChannel(f, a, b, proposers)
	})
	id := uuid.New()
	getListeners := lo.Map(bkrChans, func(b *BKRChannel, _ int) chan chan [][]byte {
		return make(chan chan [][]byte, 1)
	})
	outputListeners := make([]chan [][]byte, len(bkrChans))
	for i, b := range bkrChans {
		proposal := []byte(fmt.Sprintf("Hello World %d", i))
		go func() {
			delay := time.Duration(rand.IntN(int(maxDelay)))
			time.Sleep(delay * time.Millisecond)
			outputListener, err := b.Propose(id, proposal)
			assert.NoError(t, err)
			getListeners[i] <- outputListener
		}()
	}
	for i, l := range getListeners {
		outputListeners[i] = <-l
	}
	results := lo.Map(outputListeners, func(o chan [][]byte, _ int) [][]byte {
		return <-o
	})
	assert.True(t, len(results) >= int(n-f))
	firstResult := results[0]
	assert.True(t, lo.EveryBy(results, func(r [][]byte) bool { return equalsOutputs(r, firstResult) }))
	assert.True(t, lo.EveryBy(nodes, func(n *on.Node) bool { return n.Close() == nil }))
}
