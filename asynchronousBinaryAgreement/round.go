package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"log/slog"
)

var roundLogger = utils.GetLogger("MMR Round", slog.LevelWarn)

type mmrRound struct {
	bindingCrusaderAgreement
	coinReqChan        chan struct{}
	coinReceiveChan    chan byte
	internalTransition chan roundTransitionResult
}

func newMMRRound(n, f uint) *mmrRound {
	round := &mmrRound{
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

func (r *mmrRound) execRound() {
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

func (r *mmrRound) submitCoin(coin byte) roundTransitionResult {
	if !isInputValid(coin) {
		return roundTransitionResult{err: fmt.Errorf("invalid input %d", coin)}
	}
	r.coinReceiveChan <- coin
	return <-r.internalTransition
}
