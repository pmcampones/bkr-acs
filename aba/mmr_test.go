package aba

import (
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
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
	testShouldDecideMultipleNoFailuresAllProposeSame(t, 0)
}

func TestShouldDecideMultipleNoFailuresAllPropose1(t *testing.T) {
	testShouldDecideMultipleNoFailuresAllProposeSame(t, 1)
}

func testShouldDecideMultipleNoFailuresAllProposeSame(t *testing.T, proposal byte) {
	n := 10
	s := newOrderedMMRScheduler()
	nodes := lo.Map(lo.Range(n), func(_ int, _ int) uuid.UUID { return uuid.New() })
	wmmrs := lo.Map(nodes, func(node uuid.UUID, _ int) *wrappedMMR { return s.getChannels(t, uint(n), 0, node) })
	for _, wmmr := range wmmrs {
		assert.NoError(t, wmmr.m.propose(proposal))
	}
	finalDecisions := lo.Map(wmmrs, func(wmmr *wrappedMMR, _ int) byte { return <-wmmr.decision })
	assert.True(t, lo.EveryBy(finalDecisions, func(decision byte) bool { return decision == proposal }))
	for _, wmmr := range wmmrs {
		wmmr.m.close()
	}
}
