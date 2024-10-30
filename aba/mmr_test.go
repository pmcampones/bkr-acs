package aba

import (
	"github.com/google/uuid"
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
