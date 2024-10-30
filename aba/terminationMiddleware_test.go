package aba

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"pace/brb"
	on "pace/overlayNetwork"
	"testing"
)

func TestMarshalAndUnmarshalTerminationMessage(t *testing.T) {
	abaInstance := uuid.New()
	round := uint16(42)
	decision := byte(1)
	node := on.GetNode(t, "localhost:6000", "localhost:6000")
	bebChannel := on.CreateBEBChannel(node, 't')
	brbChannel := brb.CreateBRBChannel(1, 0, bebChannel, make(chan brb.BRBMsg))
	on.InitializeNodes(t, []*on.Node{node})
	m := newTerminationMiddleware(brbChannel)
	assert.NoError(t, m.broadcastDecision(abaInstance, round, decision))
	tm := <-m.output
	assert.Equal(t, abaInstance, tm.instance)
	assert.Equal(t, round, tm.round)
	assert.Equal(t, decision, tm.decision)
	assert.NotEqual(t, nil, tm.sender)
	m.close()
}
