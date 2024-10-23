package raba

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRoundShouldRejectInvalidEstimate(t *testing.T) {
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	assert.Error(t, r.proposeEstimate(2))
	r.close()
}

func TestRoundShouldRejectInvalidBVal(t *testing.T) {
	someId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	assert.Error(t, r.submitBVal(2, someId))
	r.close()
}

func TestRoundShouldRejectInvalidAux(t *testing.T) {
	someId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	assert.Error(t, r.submitAux(2, someId))
	r.close()
}

func TestRoundShouldWaitForCoinRequest(t *testing.T) {
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	transition := r.submitCoin(0)
	assert.Error(t, transition.err)
	r.close()
}

func TestRoundShouldRejectInvalidCoin(t *testing.T) {
	r := followSingleNodeCommonPath(t, 0)
	transition := r.submitCoin(2)
	assert.Error(t, transition.err)
	r.close()
}

func TestRoundShouldDecideOwnEstimate0Coin0(t *testing.T) {
	r := followSingleNodeCommonPath(t, 0)
	transition := r.submitCoin(0)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(0), transition.estimate)
	assert.True(t, transition.decided)
	r.close()
}

func TestRoundShouldNotDecideOwnEstimate0Coin1(t *testing.T) {
	r := followSingleNodeCommonPath(t, 0)
	transition := r.submitCoin(1)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(0), transition.estimate)
	assert.False(t, transition.decided)
	r.close()
}

func TestRoundShouldDecideOwnEstimate1Coin1(t *testing.T) {
	r := followSingleNodeCommonPath(t, 1)
	transition := r.submitCoin(1)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(1), transition.estimate)
	assert.True(t, transition.decided)
	r.close()
}

func TestRoundShouldNotDecideOwnEstimate1Coin0(t *testing.T) {
	r := followSingleNodeCommonPath(t, 1)
	transition := r.submitCoin(0)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(1), transition.estimate)
	assert.False(t, transition.decided)
	r.close()
}

func followSingleNodeCommonPath(t *testing.T, est byte) *round {
	myId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.proposeEstimate(est))
	bVal := <-bValChan
	assert.Equal(t, est, bVal)
	assert.NoError(t, r.submitBVal(bVal, myId))
	aux := <-auxChan
	assert.Equal(t, est, aux)
	assert.NoError(t, r.submitAux(aux, myId))
	<-coinRequest
	return r
}
