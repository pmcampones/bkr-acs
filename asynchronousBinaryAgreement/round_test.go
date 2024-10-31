package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRoundShouldRejectInvalidEstimate(t *testing.T) {
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
	assert.Error(t, r.propose(2))
	r.close()
}

func TestRoundShouldRejectInvalidBVal(t *testing.T) {
	someId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
	assert.Error(t, r.submitBVal(2, someId))
	r.close()
}

func TestRoundShouldRejectInvalidAux(t *testing.T) {
	someId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
	assert.Error(t, r.submitAux(2, someId))
	r.close()
}

func TestRoundShouldRejectRepeatedAux(t *testing.T) {
	sender := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.submitAux(0, sender))
	assert.Error(t, r.submitAux(0, sender))
}

func TestRoundShouldNotRejectDifferentBValSameSender(t *testing.T) {
	sender := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.submitBVal(0, sender))
	assert.NoError(t, r.submitBVal(1, sender))
}

func TestRoundShouldRejectSameBValSameSender(t *testing.T) {
	sender := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.submitBVal(0, sender))
	assert.Error(t, r.submitBVal(0, sender))
	assert.NoError(t, r.submitBVal(1, sender))
	assert.Error(t, r.submitBVal(1, sender))
}

func TestRoundShouldWaitForCoinRequest(t *testing.T) {
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
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

func followSingleNodeCommonPath(t *testing.T, est byte) *mmrRound {
	myId := uuid.New()
	bValChan := make(chan byte)
	auxChan := make(chan byte)
	coinRequest := make(chan struct{})
	r := newMMRRound(1, 0, bValChan, auxChan, coinRequest)
	assert.NoError(t, r.propose(est))
	bVal := <-bValChan
	assert.Equal(t, est, bVal)
	assert.NoError(t, r.submitBVal(bVal, myId))
	aux := <-auxChan
	assert.Equal(t, est, aux, "aux should be the same as the estimate")
	assert.NoError(t, r.submitAux(aux, myId))
	<-coinRequest
	return r
}

func TestRoundShouldAllDecide0Coin0NoFaults(t *testing.T) {
	est := byte(0)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, est, true)
}

func TestRoundAllShouldDecide1Coin1NoFaults(t *testing.T) {
	est := byte(1)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, est, true)
}

func TestRoundShouldNotDecide0Coin1NoFaults(t *testing.T) {
	est := byte(0)
	coin := byte(1)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, coin, false)
}

func TestRoundShouldNotDecide1Coin0NoFaults(t *testing.T) {
	est := byte(1)
	coin := byte(0)
	numNodes := 10
	f := 0
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, coin, false)
}

func TestRoundShouldAllDecide0Coin0MaxFaults(t *testing.T) {
	est := byte(0)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, est, true)
}

func TestRoundShouldAllDecide1Coin1MaxFaults(t *testing.T) {
	est := byte(1)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, est, true)
}

func TestRoundShouldNotDecide0Coin1MaxFaults(t *testing.T) {
	est := byte(0)
	coin := byte(1)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, coin, false)
}

func TestRoundShouldNotDecide1Coin0MaxFaults(t *testing.T) {
	est := byte(1)
	coin := byte(0)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSameNoCrash(t, numNodes, f, est, coin, false)
}

func testRoundAllProposeTheSameNoCrash(t *testing.T, numNodes, f int, est, coin byte, decided bool) {
	testRoundAllProposeTheSame(t, numNodes, numNodes, f, 0, est, coin, decided)
}

func TestRoundShouldAllDecide0Coin0MaxCrash(t *testing.T) {
	est := byte(0)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, 0, est, est, true)
}

func TestRoundShouldAllDecide1Coin1MaxCrash(t *testing.T) {
	est := byte(1)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, 0, est, est, true)
}

func TestRoundShouldNotDecide0Coin1MaxCrash(t *testing.T) {
	est := byte(0)
	coin := byte(1)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, 0, est, coin, false)
}

func TestRoundShouldNotDecide1Coin0MaxCrash(t *testing.T) {
	est := byte(1)
	coin := byte(0)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, 0, est, coin, false)
}

func TestRoundShouldAllDecide0Coin0MaxByzantine(t *testing.T) {
	est := byte(0)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, f, est, est, true)
}

func TestRoundShouldAllDecide1Coin1MaxByzantine(t *testing.T) {
	est := byte(1)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, f, est, est, true)
}

func TestRoundShouldNotDecide0Coin1MaxByzantine(t *testing.T) {
	est := byte(0)
	coin := byte(1)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, f, est, coin, false)
}

func TestRoundShouldNotDecide1Coin0MaxByzantine(t *testing.T) {
	est := byte(1)
	coin := byte(0)
	f := 4
	numNodes := 3*f + 1
	testRoundAllProposeTheSame(t, numNodes-f, numNodes, f, f, est, coin, false)
}

func testRoundAllProposeTheSame(t *testing.T, correctNodes, n, f, byzantine int, est, coin byte, decided bool) {
	if correctNodes > n {
		t.Fatalf("correctNodes %d is greater than n %d. You messed the order of the arguments", correctNodes, n)
	}
	rounds, coinChans := instantiateCorrect(t, n, correctNodes, f)
	byzIds := lo.Map(lo.Range(byzantine), func(_ int, _ int) uuid.UUID { return uuid.New() })
	for _, r := range rounds {
		for _, byz := range byzIds {
			assert.NoError(t, r.submitBVal(1-est, byz))
			assert.NoError(t, r.submitAux(1-est, byz))
		}
	}
	for _, r := range rounds {
		assert.NoError(t, r.propose(est))
	}
	for _, cc := range coinChans {
		<-cc
	}
	for _, r := range rounds {
		transition := r.submitCoin(coin)
		assert.NoError(t, transition.err)
		assert.Equal(t, est, transition.estimate, "estimate for next mmrRound should be %d but was %d", est, transition.estimate)
		assert.Equal(t, decided, transition.decided, "decision should be %t but was %t", decided, transition.decided)
		r.close()
	}
}

func instantiateCorrect(t *testing.T, maxNodes, numNodes, f int) ([]*mmrRound, []chan struct{}) {
	s := newOrderedScheduler()
	rounds := make([]*mmrRound, numNodes)
	coinChans := make([]chan struct{}, numNodes)
	for i := 0; i < numNodes; i++ {
		bValChan, auxChan := s.getChannels(t, uuid.New())
		coinChan := make(chan struct{})
		r := newMMRRound(uint(maxNodes), uint(f), bValChan, auxChan, coinChan)
		s.addRound(r)
		rounds[i] = r
		coinChans[i] = coinChan
	}
	return rounds, coinChans
}
