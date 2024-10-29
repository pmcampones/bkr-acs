package aba

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"pace/brb"
	on "pace/overlayNetwork"
	"testing"
	"time"
)

func TestShouldIgnoreMessagesFromOtherABA(t *testing.T) {
	node := on.GetNode(t, "localhost:6000", "localhost:6000")
	beb := on.CreateBEBChannel(node, 'A')
	brbDeliver := make(chan brb.BRBMsg)
	brbChan := brb.CreateBRBChannel(1, 0, beb, brbDeliver)
	on.InitializeNodes(t, []*on.Node{node})
	tg := newTerminationGadget([]byte("prefix"), brbChan, 0)
	assert.NoError(t, brbChan.BRBroadcast([]byte("This message should be ignored")))
	tick := time.Tick(100 * time.Millisecond)
	select {
	case <-tg.output:
		t.Error("Should not have received a termination signal")
	case <-tick:
	}
	brbChan.Close()
	assert.NoError(t, node.Disconnect())
}

func TestShouldTerminateWithOwnDecision(t *testing.T) {
	prefix := []byte("prefix")
	node := on.GetNode(t, "localhost:6000", "localhost:6000")
	beb := on.CreateBEBChannel(node, 'A')
	brbDeliver := make(chan brb.BRBMsg)
	brbChan := brb.CreateBRBChannel(1, 0, beb, brbDeliver)
	on.InitializeNodes(t, []*on.Node{node})
	tg := newTerminationGadget(prefix, brbChan, 0)
	assert.NoError(t, brbChan.BRBroadcast(append(prefix, 0)))
	decision := <-tg.output
	assert.Equal(t, byte(0), decision)
}

func TestShouldWaitForThresholdDecisions(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	prefix := []byte("prefix")
	nodes := lo.Map(lo.Range(int(n)), func(i int, _ int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetNode(t, address, "localhost:6000")
	})
	bebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.CreateBEBChannel(node, 'A') })
	brbs := lo.Map(bebs, func(beb *on.BEBChannel, _ int) *brb.BRBChannel {
		return brb.CreateBRBChannel(n, f, beb, make(chan brb.BRBMsg))
	})
	tgs := lo.Map(brbs, func(brb *brb.BRBChannel, _ int) *terminationGadget { return newTerminationGadget(prefix, brb, f) })
	on.InitializeNodes(t, nodes)
	for _, brbChan := range brbs[:f] {
		assert.NoError(t, brbChan.BRBroadcast(append(prefix, 0)))
	}
	tick := time.Tick(100 * time.Millisecond)
	select {
	case <-tgs[0].output:
		t.Error("Should not have received a termination signal")
	case <-tick:
	}
	for _, brbChan := range brbs[f : 2*f+1] {
		assert.NoError(t, brbChan.BRBroadcast(append(prefix, 1)))
	}
	decisions := lo.Map(tgs, func(tg *terminationGadget, _ int) byte { return <-tg.output })
	assert.True(t, lo.EveryBy(decisions, func(decision byte) bool { return decision == 1 }))
}
