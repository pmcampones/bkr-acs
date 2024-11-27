package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
