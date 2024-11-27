package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var roundLogger = utils.GetLogger("MMR Round", slog.LevelWarn)

type mmrRound struct {
	n                uint
	f                uint
	sentEcho         []bool
	receivedEchoes   []map[uuid.UUID]bool
	receivedVotes    []map[uuid.UUID]bool
	echoChan         chan byte
	voteChan         chan byte
	coinReqChan      chan struct{}
	hasRequestedCoin bool
}

func newMMRRound(n, f uint) *mmrRound {
	return &mmrRound{
		n:                n,
		f:                f,
		sentEcho:         []bool{false, false},
		receivedEchoes:   []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		receivedVotes:    []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		echoChan:         make(chan byte, 2),
		voteChan:         make(chan byte, 1),
		coinReqChan:      make(chan struct{}, 1),
		hasRequestedCoin: false,
	}
}

func isInputValid(val byte) bool {
	return val == 0 || val == 1
}

func (h *mmrRound) propose(est byte) error {
	roundLogger.Info("proposing estimate", "est", est)
	if !isInputValid(est) {
		return fmt.Errorf("invalid input %d", est)
	} else if h.sentEcho[est] {
		roundLogger.Debug("already sent echo", "est", est)
	} else {
		h.broadcastEcho(est)
	}
	return nil
}

func (h *mmrRound) submitEcho(echo byte, sender uuid.UUID) error {
	if !isInputValid(echo) {
		return fmt.Errorf("invalid input %d", echo)
	} else if h.receivedEchoes[echo][sender] {
		return fmt.Errorf("duplicate echo from %s", sender)
	}
	roundLogger.Debug("submitting echo", "echo", echo, "sender", sender)
	h.receivedEchoes[echo][sender] = true
	numBval := len(h.receivedEchoes[echo])
	if numBval == int(h.f+1) && !h.sentEcho[echo] {
		h.broadcastEcho(echo)
	}
	if numBval == int(h.n-h.f) {
		if len(h.receivedEchoes[1-echo]) < int(h.n-h.f) {
			h.broadcastVote(echo)
		} else if !h.hasRequestedCoin {
			h.requestCoin()
		}
	}
	return nil
}

func (h *mmrRound) broadcastEcho(echo byte) {
	roundLogger.Info("broadcasting echo", "echo", echo)
	h.sentEcho[echo] = true
	h.echoChan <- echo
}

func (h *mmrRound) broadcastVote(vote byte) {
	roundLogger.Info("submitting vote", "vote", vote)
	h.voteChan <- vote
}

func (h *mmrRound) submitVote(vote byte, sender uuid.UUID) error {
	if !isInputValid(vote) {
		return fmt.Errorf("invalid input %d", vote)
	} else if h.receivedVotes[0][sender] || h.receivedVotes[1][sender] {
		return fmt.Errorf("duplicate vote from %s", sender)
	}
	roundLogger.Debug("submitting vote", "vote", vote, "sender", sender)
	h.receivedVotes[vote][sender] = true
	if !h.hasRequestedCoin && len(h.receivedVotes[vote]) >= int(h.n-h.f) {
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
		if len(h.receivedVotes[candidate]) >= int(h.n-h.f) {
			return roundTransitionResult{
				estimate: candidate,
				decided:  coin == candidate,
				err:      nil,
			}
		}
	}
	if len(h.receivedEchoes[0]) < int(h.n-h.f) || len(h.receivedEchoes[1]) < int(h.n-h.f) {
		return roundTransitionResult{err: fmt.Errorf("conditions to go to next round have not been met (getting here is really bad)")}
	}
	return roundTransitionResult{estimate: coin}
}
