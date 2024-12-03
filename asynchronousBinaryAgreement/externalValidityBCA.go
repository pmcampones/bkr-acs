package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var evbcaLogger = utils.GetLogger("Externally Valid Binding Crusader Agreement", slog.LevelWarn)

// Binding Crusader Agreement satisfying the External Validity property
// External Validity: If v is externally valid and 1-v is not externally valid, then no correct process decides differently from v.
// A value v is externally valid if (1) a correct process has proposed v or (2) if it was externally valid in the previous round and no party delivered 1-v.
type evbca struct {
	n                       uint
	f                       uint
	sentEchoes              []bool
	echoes                  []map[uuid.UUID]bool
	voted                   bool
	bound                   bool
	votes                   []map[uuid.UUID]bool
	binds                   []map[uuid.UUID]bool
	bcastEchoChan           chan byte
	bcastVoteChan           chan byte
	bcastBindChan           chan byte
	outputDecision          chan byte
	prevCoinChan            chan byte
	inputExternalValidChan  chan byte
	outputExternalValidChan chan byte
	approveValChan          chan byte
	bindBotChan             chan struct{}
	bindValChan             chan byte
	terminateBotChan        chan struct{}
	terminateValChan        chan byte
	unblockChan             chan struct{}
	closeChan               chan struct{}
}

func newEVBCA(n, f uint) evbca {
	e := evbca{
		n:                       n,
		f:                       f,
		sentEchoes:              []bool{false, false},
		echoes:                  []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		votes:                   []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		binds:                   []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		bcastEchoChan:           make(chan byte, 2),
		bcastVoteChan:           make(chan byte, 2),
		bcastBindChan:           make(chan byte, 2),
		outputDecision:          make(chan byte, 1),
		prevCoinChan:            make(chan byte, 1),
		inputExternalValidChan:  make(chan byte, 2),
		outputExternalValidChan: make(chan byte, 2),
		approveValChan:          make(chan byte, 4), //twice for each value
		bindBotChan:             make(chan struct{}, 1),
		bindValChan:             make(chan byte, 1),
		terminateBotChan:        make(chan struct{}, 2),
		terminateValChan:        make(chan byte, 1),
		unblockChan:             make(chan struct{}, 1),
		closeChan:               make(chan struct{}, 1),
	}
	go e.waitToVote()
	go e.waitToBind()
	go e.waitToDecide()
	return e
}

func (e evbca) propose(est, prevCoin byte) error {
	if est > bot || est < 0 {
		return fmt.Errorf("invald input %d, allowed: [0, 1, %d (⟂)]", est, bot)
	} else if !isInputValid(prevCoin) {
		return fmt.Errorf("invald coin %d, allowed [0, 1]", prevCoin)
	} else if est == prevCoin {
		e.fastForward(est)
	} else if est == bot {
		e.voteOnCoin(prevCoin)
	} else { // est != {prevCoin, bot}
		evbcaLogger.Info("estimate differs from previous coin. Following normal route")
		if e.sentEchoes[est] {
			evbcaLogger.Debug("already sent echo", "est", est)
		} else {
			e.broadcastEcho(est)
		}
		e.prevCoinChan <- prevCoin
	}
	return nil
}

func (e evbca) submitExternallyValid(val byte) {
	evbcaLogger.Info("submitting externally valid value", "val", val)
	e.inputExternalValidChan <- val
}

// We have decided est in the previous round and know that this value will be decided by all correct processes
// Broadcasting bind to end the round ASAP
// Broadcasting vote because some correct processes may have decided bot on the previous round, and we need their bind.
func (e evbca) fastForward(est byte) {
	evbcaLogger.Info("estimate equals previous coin. Fast forwarding to vote and bind", "est", est)
	e.sentEchoes[est] = true
	e.approveValChan <- est
	e.bindValChan <- est
}

// I have no intuitive idea why we are running this
func (e evbca) voteOnCoin(prevCoin byte) {
	evbcaLogger.Info("estimate is bot. Fast forwarding to vote on coin", "prevCoin", prevCoin)
	e.sentEchoes[prevCoin] = true
	e.sentEchoes[1-prevCoin] = true
	e.approveValChan <- prevCoin
}

func (e evbca) submitEcho(echo byte, sender uuid.UUID) error {
	if !isInputValid(echo) {
		return fmt.Errorf("invald input %d", echo)
	} else if e.echoes[echo][sender] {
		return fmt.Errorf("sender already submitted an echo with this value")
	}
	e.echoes[echo][sender] = true
	countEcho := len(e.echoes[echo])
	evbcaLogger.Debug("submitting echo", "echo", echo, "sender", sender, "echoes 0", len(e.echoes[0]), "echoes 1", len(e.echoes[1]))
	if countEcho == int(e.f+1) && !e.sentEchoes[echo] {
		e.broadcastEcho(echo)
	}
	if countEcho == int(e.n-e.f) {
		e.approveValChan <- echo
	}
	return nil
}

func (e evbca) submitVote(vote byte, sender uuid.UUID) error {
	if !isInputValid(vote) {
		return fmt.Errorf("invald input %d", echo)
	} else if e.votes[0][sender] || e.votes[1][sender] {
		return fmt.Errorf("sender already votes")
	}
	e.votes[vote][sender] = true
	evbcaLogger.Debug("submitting vote", "vote", vote, "sender", sender, "votes 0", len(e.votes[0]), "votes 1", len(e.votes[1]))
	if len(e.votes[vote]) == int(e.n-e.f) && len(e.votes[1-vote]) < int(e.n-e.f) {
		e.bindValChan <- vote
	}
	return nil
}

func (e evbca) submitBind(bind byte, sender uuid.UUID) error {
	if !isInputValid(bind) {
		return fmt.Errorf("invald input %d", echo)
	} else if e.binds[0][sender] || e.binds[1][sender] || e.binds[bot][sender] {
		return fmt.Errorf("sender already binds a value")
	}
	e.binds[bind][sender] = true
	evbcaLogger.Debug("submitting bind", "bind", bind, "sender", sender, "binds 0", len(e.binds[0]), "binds 1", len(e.binds[1]), "binds bot", len(e.binds[bot]))
	if len(e.binds[bind]) == int(e.n-e.f) {
		e.terminateValChan <- bind
	} else if len(e.binds[0])+len(e.binds[1])+len(e.binds[bot]) == int(e.n-e.f) {
		e.terminateBotChan <- struct{}{}
	}
	return nil
}

func (e evbca) waitForPrevCoin() {
	externalValid := bot
	prevCoin := <-e.prevCoinChan
	evbcaLogger.Info("received previous coin", "prevCoin", prevCoin)
	for prevCoin != externalValid {
		select {
		case externalValid = <-e.inputExternalValidChan:
		case <-e.unblockChan:
		}
	}
	evbcaLogger.Info("externally valid value is equal to coin", "val", externalValid)
	e.approveValChan <- externalValid
}

func (e evbca) waitToVote() {
	approvedVals := []bool{false, false}
	first := <-e.approveValChan
	e.outputExternalValidChan <- first
	evbcaLogger.Info("voting on value", "first", first)
	e.bcastVoteChan <- first
	approvedVals[first] = true
	for !approvedVals[1-first] {
		select {
		case val := <-e.approveValChan:
			approvedVals[val] = true
		case <-e.closeChan:
			evbcaLogger.Info("stop waiting for the second value to receive enough echoes")
			return
		}
	}
	evbcaLogger.Info("both values are valid")
	e.outputExternalValidChan <- 1 - first
	e.bindBotChan <- struct{}{}
	e.terminateBotChan <- struct{}{}
}

func (e evbca) waitToBind() {
	var bindVal byte
	select {
	case <-e.bindBotChan:
		bindVal = bot
	case v := <-e.bindValChan:
		bindVal = v
	}
	evbcaLogger.Info("binding value", "bindVal", bindVal)
	e.bcastBindChan <- bindVal
}

func (e evbca) waitToDecide() {
	var decision byte
	select {
	case decision = <-e.terminateValChan:
	case <-e.terminateBotChan:
		select {
		case decision = <-e.terminateValChan:
		case <-e.terminateBotChan:
			decision = bot
		}
	}
	evbcaLogger.Info("decided", "decision", decision)
	e.outputDecision <- decision
	e.unblockChan <- struct{}{}
}

func (e evbca) broadcastEcho(echo byte) {
	evbcaLogger.Info("broadcasting echo", "echo", echo)
	e.sentEchoes[echo] = true
	e.bcastEchoChan <- echo
}

func (e evbca) broadcastVote(vote byte) {
	evbcaLogger.Info("broadcasting vote", "vote", vote)
	e.voted = true
	e.bcastVoteChan <- vote
}

func (e evbca) broadcastBind(bind byte) {
	evbcaLogger.Info("broadcasting bind", "bind", bind)
	e.bound = true
	e.bcastBindChan <- bind
}

func (e evbca) getOutputDecision() chan byte {
	return e.outputDecision
}

func (e evbca) getBcastEchoChan() chan byte {
	return e.bcastEchoChan
}

func (e evbca) getBcastVoteChan() chan byte {
	return e.bcastVoteChan
}

func (e evbca) getBcastBindChan() chan byte {
	return e.bcastBindChan
}

func (e evbca) getOutputExternalValidChan() chan byte {
	return e.outputExternalValidChan
}

func (e evbca) close() {
	e.closeChan <- struct{}{}
}
