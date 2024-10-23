package raba

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var roundLogger = utils.GetLogger(slog.LevelDebug)

type round struct {
	handler   *roundHandler
	bValChan  chan byte
	commands  chan func()
	closeChan chan struct{}
}

func newRound(n, f uint, bValChan, auxChan chan byte, coinRequest chan struct{}) *round {
	r := &round{
		handler:   newRoundHandler(n, f, bValChan, auxChan, coinRequest),
		bValChan:  bValChan,
		commands:  make(chan func()),
		closeChan: make(chan struct{}),
	}
	go r.invoker()
	return r
}

func (r *round) proposeEstimate(est byte) error {
	roundLogger.Info("scheduling proposal estimate", "est", est)
	if !isInputValid(est) {
		return fmt.Errorf("invalid input %d", est)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.proposeEstimate(est)
	}
	return <-errChan
}

func (r *round) submitBVal(bVal byte, sender uuid.UUID) error {
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

func (r *round) submitAux(aux byte, sender uuid.UUID) error {
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

func (r *round) submitCoin(coin byte) roundTransitionResult {
	if !isInputValid(coin) {
		return roundTransitionResult{
			estimate: 2,
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

func (r *round) invoker() {
	for {
		select {
		case command := <-r.commands:
			command()
		case <-r.closeChan:
			roundLogger.Info("closing round instance")
			return
		}
	}
}

func (r *round) close() {
	roundLogger.Info("signaling to close round instance")
	r.closeChan <- struct{}{}
}

type roundHandler struct {
	n                uint
	f                uint
	binVals          []bool
	auxVals          []bool
	sentBVal         []bool
	receivedBVal     []map[uuid.UUID]bool
	receivedAux      map[uuid.UUID]bool
	bValChan         chan byte
	auxChan          chan byte
	coinReqChan      chan struct{}
	hasRequestedCoin bool
}

func newRoundHandler(n, f uint, bValChan, auxChan chan byte, coinReqChan chan struct{}) *roundHandler {
	return &roundHandler{
		n:                n,
		f:                f,
		binVals:          []bool{false, false},
		auxVals:          []bool{false, false},
		sentBVal:         []bool{false, false},
		receivedBVal:     []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		receivedAux:      make(map[uuid.UUID]bool),
		bValChan:         bValChan,
		auxChan:          auxChan,
		coinReqChan:      coinReqChan,
		hasRequestedCoin: false,
	}
}

func (h *roundHandler) proposeEstimate(est byte) error {
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
	if numBval := len(h.receivedBVal[bVal]); numBval == int(h.f+1) && !h.sentBVal[bVal] {
		h.broadcastBVal(bVal)
	} else if numBval == int(h.n-h.f) {
		h.binVals[bVal] = true
		roundLogger.Info("updating binVals", "bVal", bVal, "binVals", h.binVals)
		if h.binVals[0] != h.binVals[1] {
			h.broadcastAux(bVal)
		}
		if h.canRequestCoin() {
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
	if h.receivedAux[sender] {
		return fmt.Errorf("duplicate aux from %s", sender)
	}
	roundLogger.Debug("submitting aux", "aux", aux, "sender", sender)
	h.receivedAux[sender] = true
	h.auxVals[aux] = true
	if h.canRequestCoin() {
		h.requestCoin()
	}
	return nil
}

func (h *roundHandler) canRequestCoin() bool {
	values := h.computeValues()
	return !h.hasRequestedCoin && len(h.receivedAux) >= int(h.n-h.f) && len(values) > 0
}

func (h *roundHandler) requestCoin() {
	h.hasRequestedCoin = true
	roundLogger.Info("requesting coin")
	go func() { h.coinReqChan <- struct{}{} }()
}

func (h *roundHandler) submitCoin(coin byte) roundTransitionResult {
	if !h.hasRequestedCoin {
		return roundTransitionResult{
			estimate: 2,
			decided:  false,
			err:      fmt.Errorf("coin not requested"),
		}
	}
	roundLogger.Info("submitting coin", "coin", coin)
	nextEstimate := coin
	hasDecided := false
	if values := h.computeValues(); len(values) == 1 {
		nextEstimate = values[0]
		hasDecided = nextEstimate == coin
	}
	return roundTransitionResult{
		estimate: nextEstimate,
		decided:  hasDecided,
		err:      nil,
	}
}

func (h *roundHandler) computeValues() []byte {
	values := make([]byte, 0, 2)
	for i := 0; i < 2; i++ {
		if h.binVals[i] && h.auxVals[i] {
			values = append(values, byte(i))
		}
	}
	return values
}
