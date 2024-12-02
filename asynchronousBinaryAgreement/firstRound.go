package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"log/slog"
)

var roundLogger = utils.GetLogger("MMR Round", slog.LevelWarn)

type firstRound struct {
	bindingCrusaderAgreement
	coinReqChan        chan struct{}
	coinReceiveChan    chan byte
	internalTransition chan roundTransitionResult
}

func newMMRRound(n, f uint) *firstRound {
	round := &firstRound{
		bindingCrusaderAgreement: newBindingCrusaderAgreement(n, f),
		coinReqChan:              make(chan struct{}, 1),
		coinReceiveChan:          make(chan byte, 1),
		internalTransition:       make(chan roundTransitionResult, 1),
	}
	go round.execRound()
	return round
}

func isInputValid(val byte) bool {
	return val == 0 || val == 1
}

type roundTransitionResult struct {
	estimate byte
	decided  bool
	err      error
}

func (r *firstRound) execRound() {
	roundLogger.Info("executing round")
	dec := <-r.bindingCrusaderAgreement.outputDecision
	roundLogger.Info("round decided", "dec", dec)
	roundLogger.Info("requesting coin")
	r.coinReqChan <- struct{}{}
	coin := <-r.coinReceiveChan
	roundLogger.Info("received coin", "coin", coin)
	if dec == bot {
		r.internalTransition <- roundTransitionResult{estimate: coin}
	} else {
		transition := roundTransitionResult{estimate: dec}
		if coin == dec {
			transition.decided = true
		}
		r.internalTransition <- transition
	}
}

func (r *firstRound) submitCoin(coin byte) roundTransitionResult {
	if !isInputValid(coin) {
		return roundTransitionResult{err: fmt.Errorf("invalid input %d", coin)}
	}
	r.coinReceiveChan <- coin
	return <-r.internalTransition
}

func (r *firstRound) getBcastEchoChan() chan byte {
	return r.bcastEchoChan
}

func (r *firstRound) getBcastVoteChan() chan byte {
	return r.bcastVoteChan
}

func (r *firstRound) getBcastBindChan() chan byte {
	return r.bcastBindChan
}

func (r *firstRound) getCoinReqChan() chan struct{} {
	return r.coinReqChan
}
