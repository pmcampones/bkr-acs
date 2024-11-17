package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var roundLogger = utils.GetLogger("MMR Round", slog.LevelDebug)

type mmrRound struct {
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

func newMMRRound(n, f uint) *mmrRound {
	return &mmrRound{
		n:                n,
		f:                f,
		sentBVal:         []bool{false, false},
		receivedBVal:     []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		receivedAux:      []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		bValChan:         make(chan byte, 2),
		auxChan:          make(chan byte, 1),
		coinReqChan:      make(chan struct{}, 1),
		hasRequestedCoin: false,
	}
}

func isInputValid(bVal byte) bool {
	return bVal == 0 || bVal == 1
}

func (h *mmrRound) propose(est byte) error {
	roundLogger.Info("proposing estimate", "est", est)
	if !isInputValid(est) {
		return fmt.Errorf("invalid input %d", est)
	} else if h.sentBVal[est] {
		roundLogger.Debug("already sent bVal", "est", est)
	} else {
		h.broadcastBVal(est)
	}
	return nil
}

func (h *mmrRound) submitBVal(bVal byte, sender uuid.UUID) error {
	if !isInputValid(bVal) {
		return fmt.Errorf("invalid input %d", bVal)
	} else if h.receivedBVal[bVal][sender] {
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

func (h *mmrRound) broadcastBVal(bVal byte) {
	roundLogger.Info("broadcasting bVal", "bVal", bVal)
	h.sentBVal[bVal] = true
	h.bValChan <- bVal
}

func (h *mmrRound) broadcastAux(bVal byte) {
	roundLogger.Info("submitting aux", "aux", bVal)
	h.auxChan <- bVal
}

func (h *mmrRound) submitAux(aux byte, sender uuid.UUID) error {
	if !isInputValid(aux) {
		return fmt.Errorf("invalid input %d", aux)
	} else if h.receivedAux[0][sender] || h.receivedAux[1][sender] {
		return fmt.Errorf("duplicate aux from %s", sender)
	}
	roundLogger.Debug("submitting aux", "aux", aux, "sender", sender)
	h.receivedAux[aux][sender] = true
	if !h.hasRequestedCoin && len(h.receivedAux[aux]) >= int(h.n-h.f) {
		h.requestCoin()
	}
	return nil
}

func (h *mmrRound) requestCoin() {
	h.hasRequestedCoin = true
	h.coinReqChan <- struct{}{}
}

type roundTransitionResult struct {
	estimate byte
	decided  bool
	err      error
}

func (h *mmrRound) submitCoin(coin byte) roundTransitionResult {
	if !isInputValid(coin) {
		return roundTransitionResult{err: fmt.Errorf("invalid input %d", coin)}
	} else if !h.hasRequestedCoin {
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
