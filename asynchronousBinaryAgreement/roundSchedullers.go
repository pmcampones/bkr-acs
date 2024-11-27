package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

type orderedRoundScheduler struct {
	rounds    []*mmrRound
	commands  chan func()
	closeChan chan struct{}
}

func newOrderedScheduler() *orderedRoundScheduler {
	os := &orderedRoundScheduler{
		rounds:    make([]*mmrRound, 0),
		commands:  make(chan func()),
		closeChan: make(chan struct{}, 1),
	}
	go os.invoker()
	return os
}

func (os *orderedRoundScheduler) addRound(t *testing.T, sender uuid.UUID, r *mmrRound) {
	os.rounds = append(os.rounds, r)
	go os.processBVals(t, sender, r.getEchoChan())
	go os.processAux(t, sender, r.getVoteChan())
}

func (os *orderedRoundScheduler) processAux(t *testing.T, sender uuid.UUID, auxChan chan byte) {
	func() {
		aux := <-auxChan
		for _, r := range os.rounds {
			os.commands <- func() {
				assert.NoError(t, r.submitVote(aux, sender))
			}
		}
	}()
}

func (os *orderedRoundScheduler) processBVals(t *testing.T, sender uuid.UUID, bValChan chan byte) {
	func() {
		bVal := <-bValChan
		for _, r := range os.rounds {
			os.commands <- func() {
				assert.NoError(t, r.submitEcho(bVal, sender))
			}
		}
	}()
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
