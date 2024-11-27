package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldDecide1IfAllProposeSame(t *testing.T) {
	proposal := byte(1)
	testShouldDecideIfAllProposeSame(t, proposal)
}

func TestShouldDecide0IfAllProposeSame(t *testing.T) {
	proposal := byte(0)
	testShouldDecideIfAllProposeSame(t, proposal)
}

func testShouldDecideIfAllProposeSame(t *testing.T, proposal byte) {
	f := uint(3)
	n := 3*f + 1
	scheduler := newOrderedCAScheduler(t)
	instances := lo.Map(lo.Range(int(n)), func(_ int, _ int) crusaderAgreement {
		return newCrusaderAgreement(n, f)
	})
	senders := lo.Map(instances, func(_ crusaderAgreement, _ int) uuid.UUID {
		return uuid.New()
	})
	for _, tuple := range lo.Zip2(instances, senders) {
		instance, sender := tuple.Unpack()
		scheduler.addInstance(instance, sender)
	}
	for _, instance := range instances {
		assert.NoError(t, instance.propose(proposal))
	}
	decisions := lo.Map(instances, func(instance crusaderAgreement, _ int) byte {
		return <-instance.outputDecision
	})
	assert.True(t, lo.EveryBy(decisions, func(decision byte) bool {
		return decision == proposal
	}))
}
