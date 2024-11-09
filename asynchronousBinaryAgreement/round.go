package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var roundLogger = utils.GetLogger(slog.LevelWarn)

type mmrRound struct {
	handler   *roundHandler
	commands  chan func()
	closeChan chan struct{}
}

func newMMRRound(n, f uint, bValChan, auxChan chan byte, coinRequest chan struct{}) *mmrRound {
	r := &mmrRound{
		handler:   newRoundHandler(n, f, bValChan, auxChan, coinRequest),
		commands:  make(chan func()),
		closeChan: make(chan struct{}),
	}
	go r.invoker()
	return r
}

func (r *mmrRound) propose(est byte) error {
	roundLogger.Info("scheduling proposal estimate", "est", est)
	if !isInputValid(est) {
		return fmt.Errorf("invalid input %d", est)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.propose(est)
	}
	return <-errChan
}

func (r *mmrRound) submitBVal(bVal byte, sender uuid.UUID) error {
	roundLogger.Debug("scheduling submit bVal", "bVal", bVal, "sender", sender)
	if !isInputValid(bVal) {
		return fmt.Errorf("invalid input %d", bVal)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.submitBVal(bVal, sender)
	}
	return <-errChan
}

func (r *mmrRound) submitAux(aux byte, sender uuid.UUID) error {
	roundLogger.Debug("scheduling submit aux", "aux", aux, "sender", sender)
	if !isInputValid(aux) {
		return fmt.Errorf("invalid input %d", aux)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.submitAux(aux, sender)
	}
	return <-errChan
}

type roundTransitionResult struct {
	estimate byte
	decided  bool
	err      error
}

func (r *mmrRound) submitCoin(coin byte) roundTransitionResult {
	if !isInputValid(coin) {
		return roundTransitionResult{
			estimate: bot,
			decided:  false,
			err:      fmt.Errorf("invalid input %d", coin),
		}
	}
	roundLogger.Info("scheduling submit coin", "coin", coin)
	transitionChan := make(chan roundTransitionResult)
	r.commands <- func() { transitionChan <- r.handler.submitCoin(coin) }
	return <-transitionChan
}

func isInputValid(bVal byte) bool {
	return bVal == 0 || bVal == 1
}

func (r *mmrRound) invoker() {
	for {
		select {
		case command := <-r.commands:
			command()
		case <-r.closeChan:
			roundLogger.Info("closing mmrRound instance")
			return
		}
	}
}

func (r *mmrRound) close() {
	roundLogger.Info("signaling to close mmrRound instance")
	r.closeChan <- struct{}{}
}

type roundHandler struct {
	n                uint
	f                uint
	sentBVal         []bool
	receivedBVal     []map[uuid.UUID]bool
	receivedAux      []map[uuid.UUID]bool
	bValChan         chan byte
	auxChan          chan byte
	coinReqChan      chan struct{}
	hasRequestedCoin bool
}

func newRoundHandler(n, f uint, bValChan, auxChan chan byte, coinReqChan chan struct{}) *roundHandler {
	return &roundHandler{
		n:                n,
		f:                f,
		sentBVal:         []bool{false, false},
		receivedBVal:     []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		receivedAux:      []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		bValChan:         bValChan,
		auxChan:          auxChan,
		coinReqChan:      coinReqChan,
		hasRequestedCoin: false,
	}
}

func (h *roundHandler) propose(est byte) error {
	roundLogger.Info("proposing estimate", "est", est)
	if h.sentBVal[est] {
		roundLogger.Debug("already sent bVal", "est", est)
	} else {
		h.broadcastBVal(est)
	}
	return nil
}

func (h *roundHandler) submitBVal(bVal byte, sender uuid.UUID) error {
	if h.receivedBVal[bVal][sender] {
		return fmt.Errorf("duplicate bVal from %s", sender)
	}
	roundLogger.Debug("submitting bVal", "bVal", bVal, "sender", sender)
	h.receivedBVal[bVal][sender] = true
	numBval := len(h.receivedBVal[bVal])
	if numBval == int(h.f+1) && !h.sentBVal[bVal] {
		h.broadcastBVal(bVal)
	}
	if numBval == int(h.n-h.f) {
		if len(h.receivedBVal[1-bVal]) < int(h.n-h.f) {
			h.broadcastAux(bVal)
		} else if !h.hasRequestedCoin {
			h.requestCoin()
		}
	}
	return nil
}

func (h *roundHandler) broadcastBVal(bVal byte) {
	roundLogger.Info("broadcasting bVal", "bVal", bVal)
	h.sentBVal[bVal] = true
	go func() { h.bValChan <- bVal }()
}

func (h *roundHandler) broadcastAux(bVal byte) {
	roundLogger.Info("submitting aux", "aux", bVal)
	go func() { h.auxChan <- bVal }()
}

func (h *roundHandler) submitAux(aux byte, sender uuid.UUID) error {
	if h.receivedAux[0][sender] || h.receivedAux[1][sender] {
		return fmt.Errorf("duplicate aux from %s", sender)
	}
	roundLogger.Debug("submitting aux", "aux", aux, "sender", sender)
	h.receivedAux[aux][sender] = true
	if !h.hasRequestedCoin && len(h.receivedAux[aux]) >= int(h.n-h.f) {
		h.requestCoin()
	}
	return nil
}

func (h *roundHandler) requestCoin() {
	h.hasRequestedCoin = true
	h.coinReqChan <- struct{}{}
}

func (h *roundHandler) submitCoin(coin byte) roundTransitionResult {
	if !h.hasRequestedCoin {
		return roundTransitionResult{err: fmt.Errorf("coin not requested")}
	}
	roundLogger.Info("submitting coin", "coin", coin)
	for _, candidate := range []byte{0, 1} {
		if len(h.receivedAux[candidate]) >= int(h.n-h.f) {
			return roundTransitionResult{
				estimate: candidate,
				decided:  coin == candidate,
				err:      nil,
			}
		}
	}
	if len(h.receivedBVal[0]) < int(h.n-h.f) || len(h.receivedBVal[1]) < int(h.n-h.f) {
		return roundTransitionResult{err: fmt.Errorf("conditions to go to next round have not been met (getting here is really bad)")}
	}
	return roundTransitionResult{estimate: coin}
}
