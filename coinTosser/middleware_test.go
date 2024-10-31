package coinTosser

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	on "pace/overlayNetwork"
	"testing"
)

func TestShouldUnmarshalCTShareMessage(t *testing.T) {
	node := on.GetNode(t, "localhost:6000", "localhost:6000")
	bebChannel := on.CreateBEBChannel(node, 'c')
	on.InitializeNodes(t, []*on.Node{node})
	deliverChan := make(chan *msg)
	m := newCTMiddleware(bebChannel, deliverChan)
	id := uuid.New()
	share := genCTShare(t)
	err := m.broadcastCTShare(id, share)
	assert.NoError(t, err)
	msg := <-deliverChan
	assert.Equal(t, id, msg.id)
	assert.True(t, arePointShareEquals(t, share.pt, msg.share.pt))
	assert.True(t, areProofsEqual(t, &share.proof, &msg.share.proof))
}
