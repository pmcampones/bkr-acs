package agreementCommonSubset

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	aba "pace/asynchronousBinaryAgreement"
	on "pace/overlayNetwork"
	"testing"
)

func TestShouldOutputOwnProposal2(t *testing.T) {
	testShouldOutputProposalsAlt(t, 1, 0)
}

func TestShouldOutputProposalsNoFaults2(t *testing.T) {
	testShouldOutputProposalsAlt(t, 10, 0)
}

func TestShouldOutputProposalsMaxFaults2(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldOutputProposalsAlt(t, n, f)
}

func testShouldOutputProposalsAlt(t *testing.T, n, f uint) {
	nodes := lo.Map(lo.Range(int(n)), func(i int, _ int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetNode(t, address, "localhost:6000")
	})
	abachans := getAbachans(t, n, f, nodes)
	id := uuid.New()
	proposers := lo.Map(nodes, func(node *on.Node, _ int) uuid.UUID { return uuid.New() })
	bkr2Instances := lo.Map(abachans, func(abachan *aba.AbaChannel, _ int) *BKR2 {
		return NewBKR2(id, f, proposers, abachan)
	})
	for _, bkr := range bkr2Instances {
		for i, participant := range proposers {
			input := []byte(fmt.Sprintf("input%d", i))
			assert.NoError(t, bkr.receiveInput(input, participant))
		}
	}
	outputs := lo.Map(bkr2Instances, func(bkr *BKR2, _ int) [][]byte { return <-bkr.output })
	assert.True(t, uint(len(outputs[0])) >= f)
	firstOutput := outputs[0]
	assert.True(t, lo.EveryBy(outputs, func(output [][]byte) bool { return equalsOutputs(output, firstOutput) }))
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Disconnect() == nil }))
}
