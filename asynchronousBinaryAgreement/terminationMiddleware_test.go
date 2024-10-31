package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	brb "pace/byzantineReliableBroadcast"
	on "pace/overlayNetwork"
	"testing"
)

func TestMarshalAndUnmarshalTerminationMessage(t *testing.T) {
	abaInstance := uuid.New()
	decision := byte(1)
	node := on.GetNode(t, "localhost:6000", "localhost:6000")
	bebChannel := on.CreateBEBChannel(node, 't')
	brbChannel := brb.CreateBRBChannel(1, 0, bebChannel)
	on.InitializeNodes(t, []*on.Node{node})
	m := newTerminationMiddleware(brbChannel)
	assert.NoError(t, m.broadcastDecision(abaInstance, decision))
	tm := <-m.output
	assert.Equal(t, abaInstance, tm.instance)
	assert.Equal(t, decision, tm.decision)
	assert.NotEqual(t, nil, tm.sender)
	m.close()
}
