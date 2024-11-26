package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMMRShouldTerminateWithOwnDecision(t *testing.T) {
	tg := newMmrTermination(1, 0)
	proposedDecision := byte(0)
	_, err := tg.submitDecision(proposedDecision, uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, proposedDecision, receiveDecision(t, tg))
	shouldTerminate(t, tg)
}

func TestMMRShouldNotDecideImmediately(t *testing.T) {
	f := uint(1)
	n := 3*f + 1
	tg := newMmrTermination(n, f)
	_, err := tg.submitDecision(byte(0), uuid.New())
	assert.NoError(t, err)
	shouldNotTerminateOrDecide(t, tg)
}

func TestShouldFilterRepeatedDecisionProposals(t *testing.T) {
	tg := newMmrTermination(2, 1)
	sender := uuid.New()
	proposedDecision := byte(0)
	_, err := tg.submitDecision(proposedDecision, sender)
	assert.NoError(t, err)
	_, err = tg.submitDecision(proposedDecision, sender)
	assert.Error(t, err)
}

func TestShouldFilterInvalidDecisions(t *testing.T) {
	tg := newMmrTermination(2, 1)
	_, err := tg.submitDecision(bot, uuid.New())
	assert.Error(t, err)
}

func TestMMRShouldWaitForThresholdDecisions(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	tg := newMmrTermination(n, f)
	for i := uint(0); i < f; i++ {
		_, err := tg.submitDecision(byte(0), uuid.New())
		assert.NoError(t, err)
		shouldNotTerminateOrDecide(t, tg)
		_, err = tg.submitDecision(byte(1), uuid.New())
		assert.NoError(t, err)
		shouldNotTerminateOrDecide(t, tg)
	}
	_, err := tg.submitDecision(byte(0), uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, byte(0), receiveDecision(t, tg))
	shouldTerminate(t, tg)
}

func receiveDecision(t *testing.T, tg *mmrTermination) byte {
	select {
	case decision := <-tg.deliverDecision:
		return decision
	case <-time.After(10 * time.Millisecond):
		assert.Fail(t, "timed out waiting for decision")
	}
	return bot
}

func shouldTerminate(t *testing.T, tg *mmrTermination) {
	select {
	case <-tg.notifyTermination:
		break
	case <-time.After(10 * time.Millisecond):
		assert.Fail(t, "timed out waiting for termination")
	}
}

func shouldNotTerminateOrDecide(t *testing.T, tg *mmrTermination) {
	select {
	case <-tg.notifyTermination:
		assert.Fail(t, "should not have terminated")
		break
	case <-tg.deliverDecision:
		assert.Fail(t, "should not have decided")
		break
	case <-time.After(10 * time.Millisecond):
		break
	}
}
