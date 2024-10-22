package coinTosser

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"pace/overlayNetwork"
	"testing"
)

func TestShouldUnmarshalCTShareMessage(t *testing.T) {
	node := overlayNetwork.GetNode(t, "localhost:6000", "localhost:6000")
	bebChannel := overlayNetwork.CreateBEBChannel(node, 'c')
	overlayNetwork.InitializeNodes(t, []*overlayNetwork.Node{node})
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
