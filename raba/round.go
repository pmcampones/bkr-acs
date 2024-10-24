package raba

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var roundLogger = utils.GetLogger(slog.LevelDebug)

const BOT byte = 2

type bValMsg struct {
	bVal byte
	maj  byte
}

type auxMsg struct {
	est byte
	aux byte
}

type roundTransitionResult struct {
	estimate byte
	decided  bool
	err      error
}

type round struct {
	handler   *roundHandler
	commands  chan func()
	closeChan chan struct{}
}

func newRound(n, f uint, bValChan chan bValMsg, auxChan chan auxMsg, coinRequest chan struct{}) *round {
	r := &round{
		handler:   newRoundHandler(n, f, bValChan, auxChan, coinRequest),
		commands:  make(chan func()),
		closeChan: make(chan struct{}),
	}
	go r.invoker()
	return r
}

func isNonBotInputValid(bVal byte) bool {
	return bVal == 0 || bVal == 1
}

func isBotInputValid(bVal byte) bool {
	return bVal <= BOT
}

func (r *round) proposeEstimate(est, maj byte) error {
	roundLogger.Info("scheduling proposal estimate", "est", est)
	if !isNonBotInputValid(est) {
		return fmt.Errorf("invalid input %d", est)
	} else if !isBotInputValid(maj) {
		return fmt.Errorf("invalid maj input %d", maj)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.proposeEstimate(est, maj)
	}
	return <-errChan
}

func (r *round) submitBVal(bVal, maj byte, sender uuid.UUID) error {
	roundLogger.Debug("scheduling submit bVal", "bVal", bVal, "sender", sender)
	if !isNonBotInputValid(bVal) {
		return fmt.Errorf("invalid input %d", bVal)
	} else if !isBotInputValid(maj) {
		return fmt.Errorf("invalid maj input %d", maj)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.submitBVal(bVal, maj, sender)
	}
	return <-errChan
}

func (r *round) submitAux(est, aux byte, sender uuid.UUID) error {
	roundLogger.Debug("scheduling submit aux", "aux", est, "sender", sender)
	if !isBotInputValid(est) {
		return fmt.Errorf("invalid est input %d", est)
	} else if !isNonBotInputValid(aux) {
		return fmt.Errorf("invalid aux input %d", aux)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.submitAux(est, aux, sender)
	}
	return <-errChan
}

func (r *round) submitCoin(coin byte) roundTransitionResult {
	if !isNonBotInputValid(coin) {
		return roundTransitionResult{
			estimate: BOT,
			decided:  false,
			err:      fmt.Errorf("invalid input %d", coin),
		}
	}
	roundLogger.Info("scheduling submit coin", "coin", coin)
	transitionChan := make(chan roundTransitionResult)
	r.commands <- func() { transitionChan <- r.handler.submitCoin(coin) }
	return <-transitionChan
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
	proposedMaj      byte
	binVals          []bool
	auxVals          []bool
	sentBVal         []bool
	majs             []bool
	receivedBVal     []map[uuid.UUID]bool
	receivedAux      map[uuid.UUID]bool
	bValChan         chan bValMsg
	auxChan          chan auxMsg
	coinReqChan      chan struct{}
	hasRequestedCoin bool
	hasProposed      bool
}

func newRoundHandler(n, f uint, bValChan chan bValMsg, auxChan chan auxMsg, coinReqChan chan struct{}) *roundHandler {
	return &roundHandler{
		n:                n,
		f:                f,
		proposedMaj:      BOT,
		binVals:          []bool{false, false},
		auxVals:          []bool{false, false},
		sentBVal:         []bool{false, false},
		majs:             []bool{false, false, false},
		receivedBVal:     []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		receivedAux:      make(map[uuid.UUID]bool),
		bValChan:         bValChan,
		auxChan:          auxChan,
		coinReqChan:      coinReqChan,
		hasRequestedCoin: false,
		hasProposed:      false,
	}
}

func (h *roundHandler) proposeEstimate(est, maj byte) error {
	roundLogger.Info("proposing estimate", "est", est, "maj", maj)
	if h.hasProposed {
		return fmt.Errorf("already proposed")
	}
	h.proposedMaj = maj
	h.hasProposed = true
	if h.sentBVal[est] {
		roundLogger.Debug("already sent bVal", "est", est)
	} else if err := h.broadcastBVal(est); err != nil {
		return fmt.Errorf("error broadcasting bVal: %w", err)
	}
	return nil
}

func (h *roundHandler) submitBVal(bVal, maj byte, sender uuid.UUID) error {
	if h.receivedBVal[bVal][sender] {
		return fmt.Errorf("duplicate bVal from %s", sender)
	}
	roundLogger.Debug("submitting bVal", "bVal", bVal, "sender", sender)
	h.receivedBVal[bVal][sender] = true
	h.majs[maj] = true
	if numBval := len(h.receivedBVal[bVal]); numBval == int(h.f+1) && !h.sentBVal[bVal] {
		if err := h.broadcastBVal(bVal); err != nil {
			return fmt.Errorf("error broadcasting bVal: %w", err)
		}
	} else if numBval == int(h.n-h.f) {
		h.binVals[bVal] = true
		roundLogger.Info("updating binVals", "bVal", bVal, "binVals", h.binVals)
		if h.binVals[0] != h.binVals[1] {
			h.broadcastAux(bVal, 0)
		}
		if h.canRequestCoin() {
			h.requestCoin()
		}
	}
	return nil
}

func (h *roundHandler) broadcastBVal(bVal byte) error {
	if !h.hasProposed {
		return fmt.Errorf("round has not proposed yet")
	}
	roundLogger.Info("broadcasting bVal", "bVal", bVal)
	h.sentBVal[bVal] = true
	bvMsg := bValMsg{
		bVal: bVal,
		maj:  h.proposedMaj,
	}
	go func() { h.bValChan <- bvMsg }()
	return nil
}

func (h *roundHandler) broadcastAux(est, aux byte) {
	roundLogger.Info("submitting aux", "est", est, "aux", aux)
	aMsg := auxMsg{
		est: est,
		aux: aux,
	}
	go func() { h.auxChan <- aMsg }()
}

func (h *roundHandler) submitAux(est, aux byte, sender uuid.UUID) error {
	if h.receivedAux[sender] {
		return fmt.Errorf("duplicate aux from %s", sender)
	}
	roundLogger.Debug("submitting aux", "aux", est, "sender", sender)
	h.receivedAux[sender] = true
	h.auxVals[est] = true
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
			estimate: BOT,
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
