package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBCAShouldDecide1IfAllProposeSame(t *testing.T) {
	proposal := byte(1)
	testBCAShouldDecideIfAllProposeSame(t, proposal)
}

func TestBCAShouldDecide0IfAllProposeSame(t *testing.T) {
	proposal := byte(0)
	testBCAShouldDecideIfAllProposeSame(t, proposal)
}

func testBCAShouldDecideIfAllProposeSame(t *testing.T, proposal byte) {
	f := uint(3)
	n := 3*f + 1
	scheduler := newOrderedBCAScheduler(t)
	instances := lo.Map(lo.Range(int(n)), func(_ int, _ int) bindingCrusaderAgreement {
		return newBindingCrusaderAgreement(n, f)
	})
	senders := lo.Map(instances, func(_ bindingCrusaderAgreement, _ int) uuid.UUID {
		return uuid.New()
	})
	for _, tuple := range lo.Zip2(instances, senders) {
		instance, sender := tuple.Unpack()
		scheduler.addInstance(instance, sender)
	}
	for _, instance := range instances {
		assert.NoError(t, instance.propose(proposal))
	}
	decisions := lo.Map(instances, func(instance bindingCrusaderAgreement, _ int) byte {
		return <-instance.outputDecision
	})
	assert.True(t, lo.EveryBy(decisions, func(decision byte) bool {
		return decision == proposal
	}))
}

type orderedBCAScheduler struct {
	t         *testing.T
	instances []bindingCrusaderAgreement
	commands  chan func()
}

func newOrderedBCAScheduler(t *testing.T) *orderedBCAScheduler {
	s := &orderedBCAScheduler{
		t:         t,
		instances: make([]bindingCrusaderAgreement, 0),
		commands:  make(chan func()),
	}
	go s.invoker()
	return s
}

func (s *orderedBCAScheduler) invoker() {
	for {
		select {
		case cmd := <-s.commands:
			cmd()
		}
	}
}

func (s *orderedBCAScheduler) addInstance(ca bindingCrusaderAgreement, sender uuid.UUID) {
	s.instances = append(s.instances, ca)
	go s.listenEchoes(ca, sender)
	go s.listenVotes(ca, sender)
	go s.listenBinds(ca, sender)
}

func (s *orderedBCAScheduler) listenEchoes(ca bindingCrusaderAgreement, sender uuid.UUID) {
	for {
		echo := <-ca.bcastEchoChan
		for _, instance := range s.instances {
			s.commands <- func() {
				assert.NoError(s.t, instance.submitEcho(echo, sender))
			}
		}
	}
}

func (s *orderedBCAScheduler) listenVotes(ca bindingCrusaderAgreement, sender uuid.UUID) {
	vote := <-ca.bcastVoteChan
	for _, instance := range s.instances {
		s.commands <- func() {
			assert.NoError(s.t, instance.submitVote(vote, sender))
		}
	}
	vote2 := <-ca.bcastVoteChan
	s.t.Errorf("instance voted twice: first vote %d; second vote %d", vote, vote2)
}

func (s *orderedBCAScheduler) listenBinds(ca bindingCrusaderAgreement, sender uuid.UUID) {
	bind := <-ca.bcastBindChan
	for _, instance := range s.instances {
		s.commands <- func() {
			assert.NoError(s.t, instance.submitBind(bind, sender))
		}
	}
	bind2 := <-ca.bcastBindChan
	s.t.Errorf("instance bound twice: first bind %d; second bind %d", bind, bind2)
}
