package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"math/rand/v2"
	"testing"
)

func TestShouldDecideSelfProposedValue0(t *testing.T) {
	testShouldDecideSelfProposedValue(t, 0)
}

func TestShouldDecideSelfProposedValue1(t *testing.T) {
	testShouldDecideSelfProposedValue(t, 1)
}

func testShouldDecideSelfProposedValue(t *testing.T, proposal byte) {
	s := newMMRScheduler(0)
	myself := uuid.New()
	wmmr := s.getChannels(t, 1, 0, myself)
	assert.NoError(t, wmmr.m.propose(proposal))
	finalDecision := <-wmmr.decision
	assert.Equal(t, proposal, finalDecision)
	wmmr.m.close()
}

func TestShouldDecideMultipleNoFailuresAllPropose0Ordered(t *testing.T) {
	n := uint(10)
	testShouldDecideMultipleAllProposeSame(t, n, n, 0, 0, 0, 0)
}

func TestShouldDecideMultipleNoFailuresAllPropose0Unordered(t *testing.T) {
	n := uint(10)
	testShouldDecideMultipleAllProposeSame(t, n, n, 0, 0, 0, 100)
}

func TestShouldDecideMultipleNoFailuresAllPropose1Ordered(t *testing.T) {
	n := uint(10)
	testShouldDecideMultipleAllProposeSame(t, n, n, 0, 0, 1, 0)
}

func TestShouldDecideMultipleNoFailuresAllPropose1Unordered(t *testing.T) {
	n := uint(10)
	testShouldDecideMultipleAllProposeSame(t, n, n, 0, 0, 1, 100)
}

func TestShouldDecideMultipleMaxFailuresAllPropose0Ordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n, f, 0, 0, 0)
}

func TestShouldDecideMultipleMaxFailuresAllPropose0Unordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n, f, 0, 0, 100)
}

func TestShouldDecideMultipleMaxFailuresAllPropose1Ordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n, f, 0, 1, 0)
}

func TestShouldDecideMultipleMaxFailuresAllPropose1Unordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n, f, 0, 1, 100)
}

func TestShouldDecideMultipleMaxCrashAllPropose0Ordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, 0, 0, 0)
}

func TestShouldDecideMultipleMaxCrashAllPropose0Unordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, 0, 0, 100)
}

func TestShouldDecideMultipleMaxCrashAllPropose1Ordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, 0, 1, 0)
}

func TestShouldDecideMultipleMaxCrashAllPropose1Unordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, 0, 1, 100)
}

func TestShouldDecideMultipleMaxByzantinePropose0Ordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, f, 0, 0)
}

func TestShouldDecideMultipleMaxByzantinePropose0Unordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, f, 0, 100)
}

func TestShouldDecideMultipleMaxByzantinePropose1Ordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, f, 1, 0)
}

func TestShouldDecideMultipleMaxByzantinePropose1Unordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, f, 1, 100)
}

func testShouldDecideMultipleAllProposeSame(t *testing.T, n, correct, f, byzantine uint, proposal byte, schedulerInterval int) {
	s := newMMRScheduler(schedulerInterval)
	nodes := lo.Map(lo.Range(int(correct)), func(_ int, _ int) uuid.UUID { return uuid.New() })
	wmmrs := lo.Map(nodes, func(node uuid.UUID, _ int) *wrappedMMR { return s.getChannels(t, n, f, node) })
	byzIds := lo.Map(lo.Range(int(byzantine)), func(_ int, _ int) uuid.UUID { return uuid.New() })
	for _, byzId := range byzIds {
		for _, wmmr := range wmmrs {
			dec, err := wmmr.m.submitDecision(1-proposal, byzId)
			assert.NoError(t, err)
			assert.Equal(t, bot, dec)
			for r := 0; r < 20; r++ {
				val := rand.IntN(2)
				assert.NoError(t, wmmr.m.submitBVal(byte(val), byzId, uint16(r)))
				assert.NoError(t, wmmr.m.submitAux(byte(val), byzId, uint16(r)))
			}
		}
	}
	for _, wmmr := range wmmrs {
		assert.NoError(t, wmmr.m.propose(proposal))
	}
	finalDecisions := lo.Map(wmmrs, func(wmmr *wrappedMMR, _ int) byte { return <-wmmr.decision })
	assert.True(t, lo.EveryBy(finalDecisions, func(decision byte) bool { return decision == proposal }))
	//for _, wmmr := range wmmrs {
	//	wmmr.m.close()
	//}
}

func TestShouldDecideMultipleNoFailuresDifferentProposalsOrdered(t *testing.T) {
	n := uint(10)
	testShouldDecideSameDifferentProposals(t, n, n, 0, 0, 0)
}

func TestShouldDecideMultipleNoFailuresDifferentProposalsUnordered(t *testing.T) {
	n := uint(10)
	testShouldDecideSameDifferentProposals(t, n, n, 0, 0, 100)
}

func TestShouldDecideMultipleMaxFailuresDifferentProposalsOrdered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideSameDifferentProposals(t, n, n, f, 0, 0)
}

func TestShouldDecideMultipleMaxFailuresDifferentProposalsUnordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideSameDifferentProposals(t, n, n, f, 0, 100)
}

func TestShouldDecideMultipleMaxCrashDifferentProposalsOrdered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	correct := n - f
	testShouldDecideSameDifferentProposals(t, n, correct, f, 0, 0)
}

func TestShouldDecideMultipleMaxCrashDifferentProposalsUnordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	correct := n - f
	testShouldDecideSameDifferentProposals(t, n, correct, f, 0, 100)
}

func TestShouldDecideMultipleMaxByzantineDifferentProposalsOrdered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	correct := n - f
	byzantine := f
	testShouldDecideSameDifferentProposals(t, n, correct, f, byzantine, 0)
}

func TestShouldDecideMultipleMaxByzantineDifferentProposalsUnordered(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	correct := n - f
	byzantine := f
	testShouldDecideSameDifferentProposals(t, n, correct, f, byzantine, 100)
}

func testShouldDecideSameDifferentProposals(t *testing.T, n uint, correct uint, f uint, byzantine uint, schedulerInterval int) {
	s := newMMRScheduler(schedulerInterval)
	proposals := lo.Map(lo.Range(int(correct)), func(i int, _ int) byte { return byte(rand.IntN(2)) })
	nodes := lo.Map(lo.Range(int(correct)), func(_ int, _ int) uuid.UUID { return uuid.New() })
	byzIds := lo.Map(lo.Range(int(byzantine)), func(_ int, _ int) uuid.UUID { return uuid.New() })
	wmmrs := lo.Map(nodes, func(node uuid.UUID, _ int) *wrappedMMR { return s.getChannels(t, n, f, node) })
	for _, byzId := range byzIds {
		for _, wmmr := range wmmrs {
			dec, err := wmmr.m.submitDecision(byte(rand.IntN(2)), byzId)
			assert.NoError(t, err)
			assert.Equal(t, bot, dec)
			for r := 0; r < 20; r++ {
				val := rand.IntN(2)
				assert.NoError(t, wmmr.m.submitBVal(byte(val), byzId, uint16(r)))
				assert.NoError(t, wmmr.m.submitAux(byte(val), byzId, uint16(r)))
			}
		}
	}
	for _, tuple := range lo.Zip2(wmmrs, proposals) {
		wmmr, proposal := tuple.Unpack()
		assert.NoError(t, wmmr.m.propose(proposal))
	}
	finalDecisions := lo.Map(wmmrs, func(wmmr *wrappedMMR, _ int) byte { return <-wmmr.decision })
	dec := finalDecisions[0]
	assert.True(t, lo.EveryBy(finalDecisions, func(decision byte) bool { return decision == dec }))
	t.Logf("Correct nodes decided on %d\n", dec)

	//for _, wmmr := range wmmrs {
	//	wmmr.m.close()
	//}
}
