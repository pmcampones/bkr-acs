package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMMRShouldTerminateWithOwnDecision(t *testing.T) {
	tg := newMmrTermination(0)
	proposedDecision := byte(0)
	decision, err := tg.submitDecision(proposedDecision, uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, proposedDecision, decision)
}

func TestMMRShouldNotDecideImmediately(t *testing.T) {
	tg := newMmrTermination(1)
	decision, err := tg.submitDecision(byte(0), uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, bot, decision)
}

func TestShouldFilterRepeatedDecisionProposals(t *testing.T) {
	tg := newMmrTermination(1)
	sender := uuid.New()
	proposedDecision := byte(0)
	_, err := tg.submitDecision(proposedDecision, sender)
	assert.NoError(t, err)
	_, err = tg.submitDecision(proposedDecision, sender)
	assert.Error(t, err)
}

func TestShouldFilterInvalidDecisions(t *testing.T) {
	tg := newMmrTermination(0)
	_, err := tg.submitDecision(bot, uuid.New())
	assert.Error(t, err)
}

func TestMMRShouldWaitForThresholdDecisions(t *testing.T) {
	f := uint(3)
	tg := newMmrTermination(f)
	for i := uint(0); i < f; i++ {
		dec0, err := tg.submitDecision(byte(0), uuid.New())
		assert.NoError(t, err)
		assert.Equal(t, bot, dec0)
		dec1, err := tg.submitDecision(byte(1), uuid.New())
		assert.NoError(t, err)
		assert.Equal(t, bot, dec1)
	}
	decision, err := tg.submitDecision(byte(0), uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, byte(0), decision)
}
