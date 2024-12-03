package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var bcaLogger = utils.GetLogger("Alternative Binding Crusader Agreement", slog.LevelWarn)

// Default Binding Crusader Agreement satisfying the (internal) Validity property
// Validity: If all correct processes propose v, no correct process decides differently from v.
type ivbca struct {
	n                       uint
	f                       uint
	sentEchoes              []bool
	echoes                  []map[uuid.UUID]bool
	voted                   []map[uuid.UUID]bool
	bound                   []map[uuid.UUID]bool
	bcastEchoChan           chan byte
	bcastVoteChan           chan byte
	bcastBindChan           chan byte
	outputDecision          chan byte
	outputExternalValidChan chan byte
	approveValChan          chan byte
	bindBotChan             chan struct{}
	bindValChan             chan byte
	terminateBotChan        chan struct{}
	terminateValChan        chan byte
	unblockChan             chan struct{}
}

func newBCA(n, f uint) ivbca {
	s := ivbca{
		n:                       n,
		f:                       f,
		sentEchoes:              []bool{false, false},
		echoes:                  []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		voted:                   []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		bound:                   []map[uuid.UUID]bool{make(map[uuid.UUID]bool), make(map[uuid.UUID]bool), make(map[uuid.UUID]bool)},
		bcastEchoChan:           make(chan byte, 2),
		bcastVoteChan:           make(chan byte, 1),
		bcastBindChan:           make(chan byte, 1),
		outputDecision:          make(chan byte, 1),
		outputExternalValidChan: make(chan byte, 2),
		approveValChan:          make(chan byte, 2),
		bindBotChan:             make(chan struct{}, 1),
		bindValChan:             make(chan byte, 1),
		terminateBotChan:        make(chan struct{}, 2),
		terminateValChan:        make(chan byte, 1),
		unblockChan:             make(chan struct{}, 1),
	}
	go s.waitToVote()
	go s.waitToBind()
	go s.waitToDecide()
	return s
}

func (s *ivbca) propose(est, prevCoin byte) error {
	if !isInputValid(est) {
		return fmt.Errorf("invald input %d", est)
	} else if prevCoin != bot {
		return fmt.Errorf("invalid input %d", prevCoin)
	} else if !s.sentEchoes[est] {
		s.broadcastEcho(est)
	}
	return nil
}

func (s *ivbca) submitExternallyValid(_ byte) {
	// do nothing
}

func (s *ivbca) submitEcho(echo byte, sender uuid.UUID) error {
	if !isInputValid(echo) {
		return fmt.Errorf("invald input %d", echo)
	} else if s.echoes[echo][sender] {
		return fmt.Errorf("sender already submitted an echo with this value")
	}
	s.echoes[echo][sender] = true
	countEcho := len(s.echoes[echo])
	bcaLogger.Debug("submitting echo", "echo", echo, "sender", sender, "echoes 0", len(s.echoes[0]), "echoes 1", len(s.echoes[1]))
	if countEcho == int(s.f+1) && !s.sentEchoes[echo] {
		s.broadcastEcho(echo)
	}
	if countEcho == int(s.n-s.f) {
		s.outputExternalValidChan <- echo
		s.approveValChan <- echo
	}
	return nil
}

func (s *ivbca) submitVote(vote byte, sender uuid.UUID) error {
	if !isInputValid(vote) {
		return fmt.Errorf("invald input %d", echo)
	} else if s.voted[0][sender] || s.voted[1][sender] {
		return fmt.Errorf("sender already voted")
	}
	s.voted[vote][sender] = true
	bcaLogger.Debug("submitting vote", "vote", vote, "sender", sender, "votes 0", len(s.voted[0]), "votes 1", len(s.voted[1]))
	if len(s.voted[vote]) == int(s.n-s.f) && len(s.voted[1-vote]) < int(s.n-s.f) {
		s.bindValChan <- vote
	}
	return nil
}

func (s *ivbca) submitBind(bind byte, sender uuid.UUID) error {
	if bind > bot || bind < 0 {
		return fmt.Errorf("invald input %d", bind)
	} else if s.bound[0][sender] || s.bound[1][sender] || s.bound[bot][sender] {
		return fmt.Errorf("sender already bound a value")
	}
	s.bound[bind][sender] = true
	bcaLogger.Debug("submitting bind", "bind", bind, "sender", sender, "binds 0", len(s.bound[0]), "binds 1", len(s.bound[1]), "binds bot", len(s.bound[bot]))
	if len(s.bound[bind]) == int(s.n-s.f) {
		s.terminateValChan <- bind
	} else if len(s.bound[0])+len(s.bound[1])+len(s.bound[bot]) == int(s.n-s.f) {
		s.terminateBotChan <- struct{}{}
	}
	return nil
}

func (s *ivbca) waitToVote() {
	first := <-s.approveValChan
	bcaLogger.Info("voting on value", "first", first)
	s.bcastVoteChan <- first
	select {
	case <-s.approveValChan:
		bcaLogger.Info("both values are valid")
		s.bindBotChan <- struct{}{}
		s.terminateBotChan <- struct{}{}
	case <-s.unblockChan:
		bcaLogger.Info("stop waiting for the second value to receive enough echoes")
	}
}

func (s *ivbca) waitToBind() {
	var bindVal byte
	select {
	case <-s.bindBotChan:
		bindVal = bot
	case v := <-s.bindValChan:
		bindVal = v
	}
	bcaLogger.Info("binding value", "bindVal", bindVal)
	s.bcastBindChan <- bindVal
}

func (s *ivbca) waitToDecide() {
	var decision byte
	select {
	case decision = <-s.terminateValChan:
	case <-s.terminateBotChan:
		select {
		case decision = <-s.terminateValChan:
		case <-s.terminateBotChan:
			decision = bot
		}
	}
	bcaLogger.Info("decided", "decision", decision)
	s.outputDecision <- decision
	s.unblockChan <- struct{}{}
}

func (s *ivbca) broadcastEcho(echo byte) {
	bcaLogger.Info("broadcasting echo", "echo", echo)
	s.sentEchoes[echo] = true
	s.bcastEchoChan <- echo
}

func (s *ivbca) getOutputDecision() chan byte {
	return s.outputDecision
}

func (s *ivbca) getBcastEchoChan() chan byte {
	return s.bcastEchoChan
}

func (s *ivbca) getBcastVoteChan() chan byte {
	return s.bcastVoteChan
}

func (s *ivbca) getBcastBindChan() chan byte {
	return s.bcastBindChan
}

func (s *ivbca) getOutputExternalValidChan() chan byte {
	return s.outputExternalValidChan
}
