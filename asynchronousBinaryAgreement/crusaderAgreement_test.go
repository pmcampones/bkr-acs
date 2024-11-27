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

type orderedCAScheduler struct {
	t         *testing.T
	instances []crusaderAgreement
	commands  chan func()
}

func newOrderedCAScheduler(t *testing.T) *orderedCAScheduler {
	s := &orderedCAScheduler{
		t:         t,
		instances: make([]crusaderAgreement, 0),
		commands:  make(chan func()),
	}
	go s.invoker()
	return s
}

func (s *orderedCAScheduler) invoker() {
	for {
		select {
		case cmd := <-s.commands:
			cmd()
		}
	}
}

func (s *orderedCAScheduler) addInstance(ca crusaderAgreement, sender uuid.UUID) {
	s.instances = append(s.instances, ca)
	go s.listenEchoes(ca, sender)
	go s.listenVotes(ca, sender)
}

func (s *orderedCAScheduler) listenEchoes(ca crusaderAgreement, sender uuid.UUID) {
	for {
		echo := <-ca.bcastEchoChan
		for _, instance := range s.instances {
			s.commands <- func() {
				assert.NoError(s.t, instance.submitEcho(echo, sender))
			}
		}
	}
}

func (s *orderedCAScheduler) listenVotes(ca crusaderAgreement, sender uuid.UUID) {
	vote := <-ca.bcastVoteChan
	for _, instance := range s.instances {
		s.commands <- func() {
			assert.NoError(s.t, instance.submitVote(vote, sender))
		}
	}
	vote2 := <-ca.bcastVoteChan
	s.t.Errorf("instance voted twice: first vote %d; second vote %d", vote, vote2)
}
