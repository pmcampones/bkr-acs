package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

type orderedRoundScheduler struct {
	rounds    []*firstRound
	commands  chan func()
	closeChan chan struct{}
}

func newOrderedScheduler() *orderedRoundScheduler {
	os := &orderedRoundScheduler{
		rounds:    make([]*firstRound, 0),
		commands:  make(chan func()),
		closeChan: make(chan struct{}, 1),
	}
	go os.invoker()
	return os
}

func (os *orderedRoundScheduler) addRound(t *testing.T, sender uuid.UUID, r *firstRound) {
	os.rounds = append(os.rounds, r)
	go os.processEchoes(t, sender, r.getBcastEchoChan())
	go os.processVotes(t, sender, r.getBcastVoteChan())
	go os.processBinds(t, sender, r.getBcastBindChan())
}

func (os *orderedRoundScheduler) processEchoes(t *testing.T, sender uuid.UUID, echoChan chan byte) {
	echo := <-echoChan
	for _, r := range os.rounds {
		os.commands <- func() {
			assert.NoError(t, r.submitEcho(echo, sender))
		}
	}
}

func (os *orderedRoundScheduler) processVotes(t *testing.T, sender uuid.UUID, voteChan chan byte) {
	vote := <-voteChan
	for _, r := range os.rounds {
		os.commands <- func() {
			assert.NoError(t, r.submitVote(vote, sender))
		}
	}
}

func (os *orderedRoundScheduler) processBinds(t *testing.T, sender uuid.UUID, bindChan chan byte) {
	bind := <-bindChan
	for _, r := range os.rounds {
		os.commands <- func() {
			assert.NoError(t, r.submitBind(bind, sender))
		}
	}
}

func (os *orderedRoundScheduler) invoker() {
	for {
		select {
		case cmd := <-os.commands:
			cmd()
		case <-os.closeChan:
			return
		}
	}
}

func (os *orderedRoundScheduler) close() {
	os.closeChan <- struct{}{}
}
