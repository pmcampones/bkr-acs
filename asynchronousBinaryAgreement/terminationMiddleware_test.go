package asynchronousBinaryAgreement

import (
	on "bkr-acs/overlayNetwork"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMarshalAndUnmarshalTerminationMessage(t *testing.T) {
	abaInstance := uuid.New()
	decision := byte(1)
	node := on.GetTestNode(t, "localhost:6000", "localhost:6000")
	bebChannel := on.NewBEBChannel(node, 't')
	on.InitializeNodes(t, []*on.Node{node})
	m := newTerminationMiddleware(bebChannel)
	assert.NoError(t, m.broadcastDecision(abaInstance, decision))
	tm := <-m.output
	assert.Equal(t, abaInstance, tm.instance)
	assert.Equal(t, decision, tm.decision)
	assert.NotEqual(t, nil, tm.sender)
	m.close()
}
