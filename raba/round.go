package raba

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var roundLogger = utils.GetLogger(slog.LevelWarn)

type round struct {
	handler   *roundHandler
	bValChan  chan byte
	commands  chan func()
	closeChan chan struct{}
}

func newRound(n, f uint, bValChan, auxChan, decideChan chan byte, coinRequest chan struct{}) *round {
	r := &round{
		handler:   newRoundHandler(n, f, bValChan, auxChan, decideChan, coinRequest),
		bValChan:  bValChan,
		commands:  make(chan func()),
		closeChan: make(chan struct{}),
	}
	go r.invoker()
	return r
}

func (r *round) proposeEstimate(est byte) error {
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
	if !isInputValid(aux) {
		return fmt.Errorf("invalid input %d", aux)
	}
	errChan := make(chan error)
	r.commands <- func() {
		errChan <- r.handler.submitAux(aux, sender)
	}
	return <-errChan
}

func (r *round) submitCoin(coin byte) (byte, error) {
	if !isInputValid(coin) {
		return 2, fmt.Errorf("invalid input %d", coin)
	}
	nxtEstimate := make(chan byte)
	r.commands <- func() {
		nxtEstimate <- r.handler.submitCoin(coin)
	}
	return <-nxtEstimate, nil
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
	decideChan       chan byte
	coinReqChan      chan struct{}
	hasRequestedCoin bool
}

func newRoundHandler(n, f uint, bValChan, auxChan, decideChan chan byte, coinReqChan chan struct{}) *roundHandler {
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
		decideChan:       decideChan,
		coinReqChan:      coinReqChan,
		hasRequestedCoin: false,
	}
}

func (h *roundHandler) proposeEstimate(est byte) error {
	if h.sentBVal[est] {
		return fmt.Errorf("already broadcast bVal %d", est)
	}
	h.broadcastBVal(est)
	return nil
}

func (h *roundHandler) submitBVal(bVal byte, sender uuid.UUID) error {
	if h.receivedBVal[bVal][sender] {
		return fmt.Errorf("duplicate bVal from %s", sender)
	}
	h.receivedBVal[bVal][sender] = true
	if numBval := len(h.receivedBVal[bVal]); numBval == int(h.f+1) && !h.sentBVal[bVal] {
		h.broadcastBVal(bVal)
	} else if numBval == int(h.n-h.f) {
		h.binVals[bVal] = true
		if h.binVals[0] != h.binVals[1] {
			go func() { h.auxChan <- bVal }()
		}
		if h.canRequestCoin() {
			h.requestCoin()
		}
	}
	return nil
}

func (h *roundHandler) broadcastBVal(bVal byte) {
	h.sentBVal[bVal] = true
	go func() { h.bValChan <- bVal }()
}

func (h *roundHandler) submitAux(aux byte, sender uuid.UUID) error {
	if h.receivedAux[sender] {
		return fmt.Errorf("duplicate aux from %s", sender)
	}
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
	go func() { h.coinReqChan <- struct{}{} }()
}

func (h *roundHandler) submitCoin(coin byte) byte {
	nextEstimate := coin
	values := h.computeValues()
	if len(values) == 1 {
		nextEstimate = values[0]
		if nextEstimate == coin {
			go func() { h.decideChan <- coin }()
		}
	}
	return nextEstimate
}

func (h *roundHandler) computeValues() []byte {
	values := make([]byte, 0)
	for i := 0; i < 2; i++ {
		if h.binVals[i] && h.auxVals[i] {
			values = append(values, byte(i))
		}
	}
	return values
}
