package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var simpleBCALogger = utils.GetLogger("Simple Binding Crusader Agreement", slog.LevelDebug)

type externallyValidBCA struct {
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

func newExternallyValidBCA(n, f uint) externallyValidBCA {
	s := externallyValidBCA{
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
	go s.waitToVote()
	go s.waitToBind()
	go s.waitToDecide()
	return s
}

func (e *externallyValidBCA) submitExternallyValid(val byte) {
	simpleBCALogger.Info("submitting externally valid value", "val", val)
	e.inputExternalValidChan <- val
}

func (e *externallyValidBCA) propose(est, prevCoin byte) error {
	if est > bot || est < 0 {
		return fmt.Errorf("invald input %d, allowed: [0, 1, %d (⟂)]", est, bot)
	} else if !isInputValid(prevCoin) {
		return fmt.Errorf("invald coin %d, allowed [0, 1]", prevCoin)
	} else if est == prevCoin {
		e.fastForward(est)
	} else if est == bot {
		e.voteOnCoin(prevCoin)
	} else { // est != {prevCoin, bot}
		simpleBCALogger.Info("estimate differs from previous coin. Following normal route")
		if e.sentEchoes[est] {
			simpleBCALogger.Debug("already sent echo", "est", est)
		} else {
			e.broadcastEcho(est)
		}
		e.prevCoinChan <- prevCoin
	}
	return nil
}

// We have decided est in the previous round and know that this value will be decided by all correct processes
// Broadcasting bind to end the round ASAP
// Broadcasting vote because some correct processes may have decided bot on the previous round, and we need their bind.
func (e *externallyValidBCA) fastForward(est byte) {
	simpleBCALogger.Info("estimate equals previous coin. Fast forwarding to vote and bind", "est", est)
	e.sentEchoes[est] = true
	e.approveValChan <- est
	e.bindValChan <- est
}

// I have no intuitive idea why we are running this
func (e *externallyValidBCA) voteOnCoin(prevCoin byte) {
	simpleBCALogger.Info("estimate is bot. Fast forwarding to vote on coin", "prevCoin", prevCoin)
	e.sentEchoes[prevCoin] = true
	e.sentEchoes[1-prevCoin] = true
	e.approveValChan <- prevCoin
}

func (e *externallyValidBCA) submitEcho(echo byte, sender uuid.UUID) error {
	if !isInputValid(echo) {
		return fmt.Errorf("invald input %d", echo)
	} else if e.echoes[echo][sender] {
		return fmt.Errorf("sender already submitted an echo with this value")
	}
	e.echoes[echo][sender] = true
	countEcho := len(e.echoes[echo])
	simpleBCALogger.Debug("submitting echo", "echo", echo, "sender", sender, "echoes 0", len(e.echoes[0]), "echoes 1", len(e.echoes[1]))
	if countEcho == int(e.f+1) && !e.sentEchoes[echo] {
		e.broadcastEcho(echo)
	}
	if countEcho == int(e.n-e.f) {
		e.approveValChan <- echo
	}
	return nil
}

func (e *externallyValidBCA) submitVote(vote byte, sender uuid.UUID) error {
	if !isInputValid(vote) {
		return fmt.Errorf("invald input %d", echo)
	} else if e.votes[0][sender] || e.votes[1][sender] {
		return fmt.Errorf("sender already votes")
	}
	e.votes[vote][sender] = true
	simpleBCALogger.Debug("submitting vote", "vote", vote, "sender", sender, "votes 0", len(e.votes[0]), "votes 1", len(e.votes[1]))
	if len(e.votes[vote]) == int(e.n-e.f) && len(e.votes[1-vote]) < int(e.n-e.f) {
		e.bindValChan <- vote
	}
	return nil
}

func (e *externallyValidBCA) submitBind(bind byte, sender uuid.UUID) error {
	if !isInputValid(bind) {
		return fmt.Errorf("invald input %d", echo)
	} else if e.binds[0][sender] || e.binds[1][sender] || e.binds[bot][sender] {
		return fmt.Errorf("sender already binds a value")
	}
	e.binds[bind][sender] = true
	simpleBCALogger.Debug("submitting bind", "bind", bind, "sender", sender, "binds 0", len(e.binds[0]), "binds 1", len(e.binds[1]), "binds bot", len(e.binds[bot]))
	if len(e.binds[bind]) == int(e.n-e.f) {
		e.terminateValChan <- bind
	} else if len(e.binds[0])+len(e.binds[1])+len(e.binds[bot]) == int(e.n-e.f) {
		e.terminateBotChan <- struct{}{}
	}
	return nil
}

func (e *externallyValidBCA) waitForPrevCoin() {
	externalValid := bot
	prevCoin := <-e.prevCoinChan
	simpleBCALogger.Info("received previous coin", "prevCoin", prevCoin)
	for prevCoin != externalValid {
		select {
		case externalValid = <-e.inputExternalValidChan:
		case <-e.unblockChan:
		}
	}
	simpleBCALogger.Info("externally valid value is equal to coin", "val", externalValid)
	e.approveValChan <- externalValid
}

func (e *externallyValidBCA) waitToVote() {
	approvedVals := []bool{false, false}
	first := <-e.approveValChan
	e.outputExternalValidChan <- first
	simpleBCALogger.Info("voting on value", "first", first)
	e.bcastVoteChan <- first
	approvedVals[first] = true
	for !approvedVals[1-first] {
		select {
		case val := <-e.approveValChan:
			approvedVals[val] = true
		case <-e.closeChan:
			simpleBCALogger.Info("stop waiting for the second value to receive enough echoes")
			return
		}
	}
	simpleBCALogger.Info("both values are valid")
	e.outputExternalValidChan <- 1 - first
	e.bindBotChan <- struct{}{}
	e.terminateBotChan <- struct{}{}
}

func (e *externallyValidBCA) waitToBind() {
	var bindVal byte
	select {
	case <-e.bindBotChan:
		bindVal = bot
	case v := <-e.bindValChan:
		bindVal = v
	}
	simpleBCALogger.Info("binding value", "bindVal", bindVal)
	e.bcastBindChan <- bindVal
}

func (e *externallyValidBCA) waitToDecide() {
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
	simpleBCALogger.Info("decided", "decision", decision)
	e.outputDecision <- decision
	e.unblockChan <- struct{}{}
}

func (e *externallyValidBCA) broadcastEcho(echo byte) {
	simpleBCALogger.Info("broadcasting echo", "echo", echo)
	e.sentEchoes[echo] = true
	e.bcastEchoChan <- echo
}

func (e *externallyValidBCA) broadcastVote(vote byte) {
	bindingCrusaderLogger.Info("broadcasting vote", "vote", vote)
	e.voted = true
	e.bcastVoteChan <- vote
}

func (e *externallyValidBCA) broadcastBind(bind byte) {
	bindingCrusaderLogger.Info("broadcasting bind", "bind", bind)
	e.bound = true
	e.bcastBindChan <- bind
}

func (e *externallyValidBCA) close() {
	e.closeChan <- struct{}{}
}
