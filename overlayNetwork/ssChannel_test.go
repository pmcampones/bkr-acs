package overlayNetwork

import (
	"bytes"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldSSSelf(t *testing.T) {
	testShouldSSNoThreshold(t, 1)
}

func TestShouldSSTwoNodesNoThreshold(t *testing.T) {
	testShouldSSNoThreshold(t, 2)
}

func TestShouldSSManyNodesNoThreshold(t *testing.T) {
	testShouldSSNoThreshold(t, 10)
}

func testShouldSSNoThreshold(t *testing.T, numNodes int) {
	nodes := lo.Map(lo.Range(numNodes), func(_ int, i int) *Node {
		return getNode(t, fmt.Sprintf("localhost:%d", 6000+i))
	})
	InitializeNodes(t, nodes)
	ss := lo.Map(nodes, func(node *Node, _ int) *SSChannel { return makeSSChannel(t, node) })
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	commitment := []byte("commitment")
	err := ss[0].SSBroadcast(secret, 0, commitment)
	assert.NoError(t, err)
	results := lo.Map(ss, func(s *SSChannel, _ int) SSMsg { return <-s.deliverChan })
	assert.True(t, lo.EveryBy(results, func(msg SSMsg) bool { return areScalarEquals(t, secret, msg.Share.Value) }))
	assert.True(t, lo.EveryBy(results, func(msg SSMsg) bool { return bytes.Equal(commitment, msg.Commitment) }))
	assert.True(t, lo.EveryBy(nodes, func(node *Node) bool { return node.Disconnect() == nil }))
}

func TestShouldSSWithHalfThreshold(t *testing.T) {
	numNodes := 10
	testShouldSSWithThreshold(t, numNodes, numNodes/2)
}

func TestShouldSSWithFullThreshold(t *testing.T) {
	numNodes := 10
	testShouldSSWithThreshold(t, numNodes, numNodes-1)
}

func testShouldSSWithThreshold(t *testing.T, numNodes, threshold int) {
	nodes := lo.Map(lo.Range(numNodes), func(_ int, i int) *Node {
		return getNode(t, fmt.Sprintf("localhost:%d", 6000+i))
	})
	InitializeNodes(t, nodes)
	ss := lo.Map(nodes, func(node *Node, _ int) *SSChannel { return makeSSChannel(t, node) })
	secret := group.Ristretto255.NewScalar().SetUint64(42)
	commitment := []byte("commitment")
	err := ss[0].SSBroadcast(secret, uint(threshold), commitment)
	assert.NoError(t, err)
	results := lo.Map(ss, func(s *SSChannel, _ int) SSMsg { return <-s.deliverChan })
	assert.True(t, lo.EveryBy(results, func(msg SSMsg) bool { return bytes.Equal(commitment, msg.Commitment) }))
	shares := lo.Map(results, func(msg SSMsg, _ int) secretsharing.Share { return msg.Share })
	recov, err := secretsharing.Recover(uint(threshold), shares)
	assert.NoError(t, err)
	assert.True(t, areScalarEquals(t, secret, recov))
	assert.True(t, lo.EveryBy(nodes, func(node *Node) bool { return node.Disconnect() == nil }))
}

func makeSSChannel(t *testing.T, node *Node) *SSChannel {
	ss, err := CreateSSChannel(node, 's')
	assert.NoError(t, err)
	return ss
}

func areScalarEquals(t *testing.T, a, b group.Scalar) bool {
	aBytes, err := a.MarshalBinary()
	assert.NoError(t, err)
	bBytes, err := b.MarshalBinary()
	assert.NoError(t, err)
	return bytes.Equal(aBytes, bBytes)
}
