package raba

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

type orderedScheduler struct {
	rounds []*round
}

func newOrderedScheduler() *orderedScheduler {
	return &orderedScheduler{
		rounds: make([]*round, 0),
	}
}

func (os *orderedScheduler) addRound(r *round) {
	os.rounds = append(os.rounds, r)
}

func (os *orderedScheduler) getChannels(t *testing.T, sender uuid.UUID) (chan byte, chan byte) {
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	go func() {
		bVal := <-bValChan
		for _, r := range os.rounds {
			go func() { assert.NoError(t, r.submitBVal(bVal, sender)) }()
		}
	}()
	go func() {
		aux := <-auxChan
		for _, r := range os.rounds {
			go func() { assert.NoError(t, r.submitAux(aux, sender)) }()
		}
	}()
	return bValChan, auxChan
}
