package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

type orderedRoundScheduler struct {
	rounds []*mmrRound
}

func newOrderedScheduler() *orderedRoundScheduler {
	return &orderedRoundScheduler{
		rounds: make([]*mmrRound, 0),
	}
}

func (os *orderedRoundScheduler) addRound(r *mmrRound) {
	os.rounds = append(os.rounds, r)
}

func (os *orderedRoundScheduler) getChannels(t *testing.T, sender uuid.UUID) (chan byte, chan byte) {
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
