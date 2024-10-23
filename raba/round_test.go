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

func TestRoundShouldRejectRepeatedAux(t *testing.T) {
	sender := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.submitAux(0, sender))
	assert.Error(t, r.submitAux(0, sender))
}

func TestRoundShouldNotRejectDifferentBValSameSender(t *testing.T) {
	sender := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.submitBVal(0, sender))
	assert.NoError(t, r.submitBVal(1, sender))
}

func TestRoundShouldRejectSameBValSameSender(t *testing.T) {
	sender := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.submitBVal(0, sender))
	assert.Error(t, r.submitBVal(0, sender))
	assert.NoError(t, r.submitBVal(1, sender))
	assert.Error(t, r.submitBVal(1, sender))
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

func TestRoundShouldAllDecide0Coin0NoFaults(t *testing.T) {
	est := byte(0)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSame(t, numNodes, f, est, est, true)
}

func TestRoundAllDecide1Coin1NoFaults(t *testing.T) {
	est := byte(1)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSame(t, numNodes, f, est, est, true)
}

func TestRoundShouldNotDecide0Coin1NoFaults(t *testing.T) {
	est := byte(0)
	coin := byte(1)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSame(t, numNodes, f, est, coin, false)
}

func TestRoundShouldNotDecide1Coin0NoFaults(t *testing.T) {
	est := byte(1)
	coin := byte(0)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSame(t, numNodes, f, est, coin, false)
}

func testRoundAllProposeTheSame(t *testing.T, numNodes int, f int, est, coin byte, decided bool) {
	rounds, coinChans := instantiateCorrect(t, numNodes, f)
	for _, r := range rounds {
		assert.NoError(t, r.proposeEstimate(est))
	}
	for _, cc := range coinChans {
		<-cc
	}
	for _, r := range rounds {
		transition := r.submitCoin(coin)
		assert.NoError(t, transition.err)
		assert.Equal(t, est, transition.estimate)
		assert.Equal(t, decided, transition.decided)
		r.close()
	}
}

func instantiateCorrect(t *testing.T, numNodes int, f int) ([]*round, []chan struct{}) {
	s := newOrderedScheduler()
	rounds := make([]*round, numNodes)
	coinChans := make([]chan struct{}, numNodes)
	for i := 0; i < numNodes; i++ {
		bValChan, auxChan := s.getChannels(t, uuid.New())
		coinChan := make(chan struct{})
		r := newRound(uint(numNodes), uint(f), bValChan, auxChan, coinChan)
		s.addRound(r)
		rounds[i] = r
		coinChans[i] = coinChan
	}
	return rounds, coinChans
}
