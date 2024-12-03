package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"math/rand/v2"
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
	instances := lo.Map(lo.Range(int(n)), func(_ int, _ int) *ivbca {
		instance := newBCA(n, f)
		return &instance
	})
	senders := lo.Map(instances, func(_ *ivbca, _ int) uuid.UUID {
		return uuid.New()
	})
	for _, tuple := range lo.Zip2(instances, senders) {
		instance, sender := tuple.Unpack()
		scheduler.addInstance(instance, sender)
	}
	for _, instance := range instances {
		assert.NoError(t, instance.propose(proposal, bot))
	}
	decisions := lo.Map(instances, func(instance *ivbca, _ int) byte {
		return <-instance.outputDecision
	})
	assert.True(t, lo.EveryBy(decisions, func(decision byte) bool {
		return decision == proposal
	}))
}

func TestBCAShouldTerminateWithManyDifferingResponses(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	scheduler := newOrderedBCAScheduler(t)
	instances := lo.Map(lo.Range(int(n)), func(_ int, _ int) *ivbca {
		instance := newBCA(n, f)
		return &instance
	})
	senders := lo.Map(instances, func(_ *ivbca, _ int) uuid.UUID {
		return uuid.New()
	})
	for _, tuple := range lo.Zip2(instances, senders) {
		instance, sender := tuple.Unpack()
		scheduler.addInstance(instance, sender)
	}
	proposals := lo.Map(instances, func(_ *ivbca, _ int) byte {
		return byte(rand.IntN(2))
	})
	for _, tuple := range lo.Zip2(instances, proposals) {
		instance, proposal := tuple.Unpack()
		assert.NoError(t, instance.propose(proposal, bot))
	}
	decisions := lo.Map(instances, func(instance *ivbca, _ int) byte {
		return <-instance.outputDecision
	})
	fmt.Println(decisions)
}

type orderedBCAScheduler struct {
	t         *testing.T
	n         uint
	f         uint
	instances []*ivbca
	commands  chan func()
}

var bcaSchedulerLogger = utils.GetLogger("BCA Scheduler", slog.LevelDebug)

func newOrderedBCAScheduler(t *testing.T) *orderedBCAScheduler {
	s := &orderedBCAScheduler{
		t:         t,
		instances: make([]*ivbca, 0),
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

func (s *orderedBCAScheduler) addInstance(ca *ivbca, sender uuid.UUID) {
	s.instances = append(s.instances, ca)
	bcaSchedulerLogger.Info("added instance", "id", sender)
	go s.listenEchoes(ca, sender)
	go s.listenVotes(ca, sender)
	go s.listenBinds(ca, sender)
}

func (s *orderedBCAScheduler) listenEchoes(ca *ivbca, sender uuid.UUID) {
	for {
		echo := <-ca.bcastEchoChan
		bcaSchedulerLogger.Info("received echo", "echo", echo, "id", sender)
		for _, instance := range s.instances {
			s.commands <- func() {

				assert.NoError(s.t, instance.submitEcho(echo, sender))
			}
		}
	}
}

func (s *orderedBCAScheduler) listenVotes(ca *ivbca, sender uuid.UUID) {
	vote := <-ca.bcastVoteChan
	bcaSchedulerLogger.Info("received vote", "vote", vote, "id", sender)
	for _, instance := range s.instances {
		s.commands <- func() {
			assert.NoError(s.t, instance.submitVote(vote, sender))
		}
	}
	vote2 := <-ca.bcastVoteChan
	s.t.Errorf("instance voted twice: id %v; first vote %d; second vote %d", sender, vote, vote2)
}

func (s *orderedBCAScheduler) listenBinds(ca *ivbca, sender uuid.UUID) {
	bind := <-ca.bcastBindChan
	bcaSchedulerLogger.Info("received bind", "bind", bind, "id", sender)
	for _, instance := range s.instances {
		s.commands <- func() {
			assert.NoError(s.t, instance.submitBind(bind, sender))
		}
	}
	bind2 := <-ca.bcastBindChan
	s.t.Errorf("instance bound twice: id %v; first bind %d; second bind %d", sender, bind, bind2)
}
