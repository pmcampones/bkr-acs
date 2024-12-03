package agreementCommonSubset

import (
	aba "bkr-acs/asynchronousBinaryAgreement"
	ct "bkr-acs/coinTosser"
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

func synchronousProposal(t *testing.T) func(instance *bkr, input []byte, participant uuid.UUID) {
	return func(instance *bkr, input []byte, participant uuid.UUID) {
		go func() {
			assert.NoError(t, instance.receiveInput(input, participant))
		}()
	}
}

func TestShouldOutputOwnProposal(t *testing.T) {
	testShouldOutputProposals(t, 1, 0, synchronousProposal(t))
}

func TestShouldOutputProposalsNoFaults(t *testing.T) {
	testShouldOutputProposals(t, 10, 0, synchronousProposal(t))
}

func TestShouldOutputProposalsMaxFaults(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldOutputProposals(t, n, f, synchronousProposal(t))
}

func TestShouldOutputOwnProposalWithDelay(t *testing.T) {
	maxDelay := uint(300)
	testShouldOutputProposals(t, 1, 0, delayedProposal(t, maxDelay))
}

func TestShouldOutputProposalsNoFaultsWithDelay(t *testing.T) {
	maxDelay := uint(300)
	testShouldOutputProposals(t, 10, 0, delayedProposal(t, maxDelay))
}

func TestShouldOutputProposalsMaxFaultsWithDelay(t *testing.T) {
	maxDelay := uint(300)
	f := uint(3)
	n := 3*f + 1
	testShouldOutputProposals(t, n, f, delayedProposal(t, maxDelay))
}

func delayedProposal(t *testing.T, maxDelay uint) func(instance *bkr, input []byte, participant uuid.UUID) {
	return func(instance *bkr, input []byte, participant uuid.UUID) {
		go func() {
			delay := time.Duration(rand.IntN(int(maxDelay)))
			time.Sleep(delay * time.Millisecond)
			assert.NoError(t, instance.receiveInput(input, participant))
		}()
	}
}

func testShouldOutputProposals(t *testing.T, n, f uint, receiveInput func(instance *bkr, input []byte, participant uuid.UUID)) {
	nodes := lo.Map(lo.Range(int(n)), func(i int, _ int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetTestNode(t, address, "localhost:6000")
	})
	abachans := getAbachans(t, n, f, nodes)
	id := uuid.New()
	proposers := lo.Map(nodes, func(node *on.Node, _ int) uuid.UUID { return uuid.New() })
	bkrInstances := lo.Map(abachans, func(abachan *aba.AbaChannel, _ int) *bkr {
		return newBKR(id, f, proposers, abachan)
	})
	for _, bkr := range bkrInstances {
		for i, participant := range proposers {
			input := []byte(fmt.Sprintf("input%d", i))
			receiveInput(bkr, input, participant)
		}
	}
	outputs := lo.Map(bkrInstances, func(bkr *bkr, _ int) [][]byte { return <-bkr.output })
	assert.True(t, uint(len(outputs[0])) >= f)
	firstOutput := outputs[0]
	assert.True(t, lo.EveryBy(outputs, func(output [][]byte) bool { return equalsOutputs(output, firstOutput) }))
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Close() == nil }))
}

func getAbachans(t *testing.T, n uint, f uint, nodes []*on.Node) []*aba.AbaChannel {
	dealSSs := lo.Map(nodes, func(node *on.Node, _ int) *on.SSChannel { return on.NewSSChannel(node, 'd') })
	ctBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.NewBEBChannel(node, 'c') })
	mBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.NewBEBChannel(node, 'm') })
	tBebs := lo.Map(nodes, func(node *on.Node, _ int) *on.BEBChannel { return on.NewBEBChannel(node, 't') })
	on.InitializeNodes(t, nodes)
	assert.NoError(t, ct.DealSecret(dealSSs[0], ct.NewScalar(42), 2*f))
	abachans := lo.ZipBy4(dealSSs, ctBebs, mBebs, tBebs, func(dealSS *on.SSChannel, ctBeb, mBeb, tBeb *on.BEBChannel) *aba.AbaChannel {
		abachan, err := aba.NewAbaChannel(n, f, dealSS, ctBeb, mBeb, tBeb)
		assert.NoError(t, err)
		return abachan
	})
	return abachans
}

func equalsOutputs(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	return lo.EveryBy(lo.Zip2(a, b), func(tuple lo.Tuple2[[]byte, []byte]) bool {
		return slices.Equal(tuple.A, tuple.B)
	})
}
