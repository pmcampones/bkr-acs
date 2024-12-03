package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

type bca interface {
	propose(est, prevCoin byte) error
	submitExternallyValid(byte)
	submitEcho(echo byte, sender uuid.UUID) error
	submitVote(vote byte, sender uuid.UUID) error
	submitBind(bind byte, sender uuid.UUID) error
	getBcastEchoChan() chan byte
	getBcastVoteChan() chan byte
	getBcastBindChan() chan byte
	getOutputDecision() chan byte
	getOutputExternalValidChan() chan byte
}

var roundLogger = utils.GetLogger("MMR Round", slog.LevelWarn)

type mmrRound struct {
	bca
	//externallyValidBCA
	coinReqChan        chan struct{}
	coinReceiveChan    chan byte
	internalTransition chan roundTransitionResult
}

func newFirstRound(n, f uint) mmrRound {
	ca := newBCA(n, f)
	round := mmrRound{
		bca:                &ca,
		coinReqChan:        make(chan struct{}, 1),
		coinReceiveChan:    make(chan byte, 1),
		internalTransition: make(chan roundTransitionResult, 1),
	}
	go round.execRound()
	return round
}

func newRound(n, f uint) mmrRound {
	ca := newEVBCA(n, f)
	round := mmrRound{
		bca:                &ca,
		coinReqChan:        make(chan struct{}, 1),
		coinReceiveChan:    make(chan byte, 1),
		internalTransition: make(chan roundTransitionResult, 1),
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
	//dec := <-r.externallyValidBCA.outputDecision
	dec := <-r.bca.getOutputDecision()
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
