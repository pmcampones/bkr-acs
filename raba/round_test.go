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
	err := r.proposeEstimate(2)
	assert.Error(t, err)
}

func TestRoundShouldRejectInvalidBVal(t *testing.T) {
	someId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	err := r.submitBVal(2, someId)
	assert.Error(t, err)
}

func TestRoundShouldRejectInvalidAux(t *testing.T) {
	someId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	err := r.submitAux(2, someId)
	assert.Error(t, err)
}

func TestRoundShouldWaitForCoinRequest(t *testing.T) {
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	transition := r.submitCoin(0)
	assert.Error(t, transition.err)
}

func TestRoundShouldRejectInvalidCoin(t *testing.T) {
	r := followSingleNodeCommonPath(t, 0)
	transition := r.submitCoin(2)
	assert.Error(t, transition.err)
}

func TestRoundShouldDecideOwnEstimate0Coin0(t *testing.T) {
	r := followSingleNodeCommonPath(t, 0)
	transition := r.submitCoin(0)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(0), transition.estimate)
	assert.True(t, transition.decided)
}

func TestRoundShouldNotDecideOwnEstimate0Coin1(t *testing.T) {
	r := followSingleNodeCommonPath(t, 0)
	transition := r.submitCoin(1)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(0), transition.estimate)
	assert.False(t, transition.decided)
}

func TestRoundShouldDecideOwnEstimate1Coin1(t *testing.T) {
	r := followSingleNodeCommonPath(t, 1)
	transition := r.submitCoin(1)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(1), transition.estimate)
	assert.True(t, transition.decided)
}

func TestRoundShouldNotDecideOwnEstimate1Coin0(t *testing.T) {
	r := followSingleNodeCommonPath(t, 1)
	transition := r.submitCoin(0)
	assert.NoError(t, transition.err)
	assert.Equal(t, byte(1), transition.estimate)
	assert.False(t, transition.decided)
}

func followSingleNodeCommonPath(t *testing.T, est byte) *round {
	myId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	err := r.proposeEstimate(est)
	assert.NoError(t, err)
	bVal := <-bValChan
	assert.Equal(t, est, bVal)
	err = r.submitBVal(bVal, myId)
	assert.NoError(t, err)
	aux := <-auxChan
	assert.Equal(t, est, aux)
	err = r.submitAux(aux, myId)
	assert.NoError(t, err)
	<-coinRequest
	return r
}
