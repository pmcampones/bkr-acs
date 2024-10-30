package aba

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
	s := newOrderedMMRScheduler()
	myself := uuid.New()
	wmmr := s.getChannels(t, 1, 0, myself)
	assert.NoError(t, wmmr.m.propose(proposal))
	finalDecision := <-wmmr.decision
	assert.Equal(t, proposal, finalDecision)
	wmmr.m.close()
}

func TestShouldDecideMultipleNoFailuresAllPropose0(t *testing.T) {
	n := uint(10)
	testShouldDecideMultipleAllProposeSame(t, n, n, 0, 0, 0)
}

func TestShouldDecideMultipleNoFailuresAllPropose1(t *testing.T) {
	n := uint(10)
	testShouldDecideMultipleAllProposeSame(t, n, n, 0, 0, 1)
}

func TestShouldDecideMultipleMaxFailuresAllPropose0(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n, f, 0, 0)
}

func TestShouldDecideMultipleMaxFailuresAllPropose1(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n, f, 0, 1)
}

func TestShouldDecideMultipleMaxCrashAllPropose0(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, 0, 0)
}

func TestShouldDecideMultipleMaxCrashAllPropose1(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, 0, 1)
}

func TestShouldDecideMultipleMaxByzantinePropose0(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, f, 0)
}

func TestShouldDecideMultipleMaxByzantinePropose1(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testShouldDecideMultipleAllProposeSame(t, n, n-f, f, f, 1)
}

func testShouldDecideMultipleAllProposeSame(t *testing.T, n, correct, f, byzantine uint, proposal byte) {
	s := newOrderedMMRScheduler()
	nodes := lo.Map(lo.Range(int(correct)), func(_ int, _ int) uuid.UUID { return uuid.New() })
	wmmrs := lo.Map(nodes, func(node uuid.UUID, _ int) *wrappedMMR { return s.getChannels(t, n, f, node) })
	byzIds := lo.Map(lo.Range(int(byzantine)), func(_ int, _ int) uuid.UUID { return uuid.New() })
	for _, byzId := range byzIds {
		for _, wmmr := range wmmrs {
			dec, err := wmmr.m.submitDecision(byte(1-proposal), byzId)
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
