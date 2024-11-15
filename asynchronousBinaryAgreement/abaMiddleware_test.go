package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	on "pace/overlayNetwork"
	"testing"
)

func TestShouldMarshalAndUnmarshalBval(t *testing.T) {
	abaInstance := uuid.New()
	round := uint16(42)
	val := byte(1)
	node := on.GetTestNode(t, "localhost:6000", "localhost:6000")
	bebChannel := on.NewBEBChannel(node, 'a')
	on.InitializeNodes(t, []*on.Node{node})
	m := newABAMiddleware(bebChannel)
	assert.NoError(t, m.broadcastBVal(abaInstance, round, val))
	amsg := <-m.output
	assert.Equal(t, bval, amsg.kind)
	assert.Equal(t, abaInstance, amsg.instance)
	assert.Equal(t, round, amsg.round)
	assert.Equal(t, val, amsg.val)
	assert.NotEqual(t, nil, amsg.sender)
	m.close()
	assert.NoError(t, node.Close())
}

func TestShouldMarshalAndUnmarshalAux(t *testing.T) {
	abaInstance := uuid.New()
	round := uint16(42)
	val := byte(1)
	node := on.GetTestNode(t, "localhost:6000", "localhost:6000")
	bebChannel := on.NewBEBChannel(node, 'a')
	on.InitializeNodes(t, []*on.Node{node})
	m := newABAMiddleware(bebChannel)
	assert.NoError(t, m.broadcastAux(abaInstance, round, val))
	amsg := <-m.output
	assert.Equal(t, aux, amsg.kind)
	assert.Equal(t, abaInstance, amsg.instance)
	assert.Equal(t, round, amsg.round)
	assert.Equal(t, val, amsg.val)
	assert.NotEqual(t, nil, amsg.sender)
	m.close()
	assert.NoError(t, node.Close())
}
