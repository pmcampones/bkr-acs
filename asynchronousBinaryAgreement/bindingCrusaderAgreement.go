package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var bindingCrusaderLogger = utils.GetLogger("Binding Crusader Agreement", slog.LevelDebug)

type bindingCrusaderAgreement struct {
	n                       uint
	f                       uint
	sentEchoes              []bool
	bound                   bool
	echoes                  []map[uuid.UUID]bool
	votes                   []map[uuid.UUID]bool
	binds                   []map[uuid.UUID]bool
	bcastEchoChan           chan byte
	bcastVoteChan           chan byte
	bcastBindChan           chan byte
	outputDecision          chan byte
	delivered               bool
	outputExternalValidChan chan byte
	bothChan                chan struct{}
	boundBotChan            chan struct{}
	botChan                 chan struct{}
	valChan                 chan byte
}

func newBindingCrusaderAgreement(n, f uint) bindingCrusaderAgreement {
	c := bindingCrusaderAgreement{
		n:                       n,
		f:                       f,
		sentEchoes:              []bool{false, false},
		bound:                   false,
		echoes:                  []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		votes:                   []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		binds:                   []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		bcastEchoChan:           make(chan byte, 2),
		bcastVoteChan:           make(chan byte, 1),
		bcastBindChan:           make(chan byte, 1),
		outputDecision:          make(chan byte, 1),
		outputExternalValidChan: make(chan byte, 2),
		delivered:               false,
		bothChan:                make(chan struct{}, 1),
		boundBotChan:            make(chan struct{}, 1),
		botChan:                 make(chan struct{}, 1),
		valChan:                 make(chan byte, 1),
	}
	bindingCrusaderLogger.Info("created new binding crusader agreement", "n", n, "f", f)
	go c.tryToOutputBot()
	go c.waitForDecision()
	return c
}

func (c *bindingCrusaderAgreement) propose(est, prevCoin byte) error {
	bindingCrusaderLogger.Info("proposing estimate", "est", est)
	if prevCoin != bot {
		return fmt.Errorf("invalid coin for non externally valid crusader agreement %d", prevCoin)
	} else if !isInputValid(est) {
		return fmt.Errorf("invalid input %d", est)
	} else if c.sentEchoes[est] {
		bindingCrusaderLogger.Debug("already sent echo", "est", est)
	} else {
		c.broadcastEcho(est)
	}
	return nil
}

func (c *bindingCrusaderAgreement) submitExternallyValid(_ byte) {
	// Do nothing. Only exists to satisfy the interface
}

func (c *bindingCrusaderAgreement) submitEcho(echo byte, sender uuid.UUID) error {
	if !isInputValid(echo) {
		return fmt.Errorf("invalid input %d", echo)
	} else if c.echoes[echo][sender] {
		return fmt.Errorf("duplicate echo from %s", sender)
	}
	c.echoes[echo][sender] = true
	bindingCrusaderLogger.Debug("submitting echo", "echo", echo, "id", sender, "echoes 0", len(c.echoes[0]), "echoes 1", len(c.echoes[1]))
	countEcho := len(c.echoes[echo])
	if countEcho == int(c.f+1) && !c.sentEchoes[echo] {
		c.broadcastEcho(echo)
	}
	if countEcho == int(c.n-c.f) {
		c.outputExternalValidChan <- echo
		countOther := len(c.echoes[1-echo])
		if countOther < int(c.n-c.f) {
			c.broadcastVote(echo)
		} else if countOther >= int(c.n-c.f) && !c.bound {
			c.broadcastBind(bot)
			c.bothChan <- struct{}{}
		}
	}
	return nil
}

func (c *bindingCrusaderAgreement) submitVote(vote byte, sender uuid.UUID) error {
	if !isInputValid(vote) {
		return fmt.Errorf("invalid input %d", vote)
	} else if c.votes[0][sender] || c.votes[1][sender] {
		return fmt.Errorf("duplicate vote from %s", sender)
	}
	c.votes[vote][sender] = true
	bindingCrusaderLogger.Debug("submitting vote", "vote", vote, "id", sender, "votes 0", len(c.votes[0]), "votes 1", len(c.votes[1]))
	countVotes := len(c.votes[vote])
	if countVotes == int(c.n-c.f) && !c.bound {
		c.broadcastBind(vote)
	}
	return nil
}

func (c *bindingCrusaderAgreement) submitBind(bind byte, sender uuid.UUID) error {
	if bind > bot || bind < 0 {
		return fmt.Errorf("invalid input %d", bind)
	} else if c.binds[0][sender] || c.binds[1][sender] || c.binds[bot][sender] {
		return fmt.Errorf("duplicate bind from %s", sender)
	}
	c.binds[bind][sender] = true
	bindingCrusaderLogger.Debug("submitting bind", "bind", bind, "id", sender, "binds 0", len(c.binds[0]), "binds 1", len(c.binds[1]), "binds bot", len(c.binds[bot]))
	if bind != bot && len(c.binds[bind]) == int(c.n-c.f) {
		c.valChan <- bind
	} else if len(c.binds[0])+len(c.binds[1])+len(c.binds[bot]) == int(c.n-c.f) {
		c.boundBotChan <- struct{}{}
	}
	return nil
}

func (c *bindingCrusaderAgreement) tryToOutputBot() {
	<-c.bothChan
	bindingCrusaderLogger.Info("both values are candidate for delivery")
	<-c.boundBotChan
	bindingCrusaderLogger.Info("received several differing bind values")
	c.botChan <- struct{}{}
}

func (c *bindingCrusaderAgreement) waitForDecision() {
	var decision byte
	select {
	case <-c.botChan:
		decision = bot
		break
	case decision = <-c.valChan:
		break
	}
	bindingCrusaderLogger.Info("delivering decision", "decision", decision)
	c.outputDecision <- decision
}

func (c *bindingCrusaderAgreement) broadcastEcho(echo byte) {
	bindingCrusaderLogger.Info("broadcasting echo", "echo", echo)
	c.sentEchoes[echo] = true
	c.bcastEchoChan <- echo
}

func (c *bindingCrusaderAgreement) broadcastVote(vote byte) {
	bindingCrusaderLogger.Info("broadcasting vote", "vote", vote)
	c.bcastVoteChan <- vote
}

func (c *bindingCrusaderAgreement) broadcastBind(bind byte) {
	bindingCrusaderLogger.Info("broadcasting bind", "bind", bind)
	c.bound = true
	c.bcastBindChan <- bind
}

func (c *bindingCrusaderAgreement) getOutputDecision() chan byte {
	return c.outputDecision
}

func (c *bindingCrusaderAgreement) getBcastEchoChan() chan byte {
	return c.bcastEchoChan
}

func (c *bindingCrusaderAgreement) getBcastVoteChan() chan byte {
	return c.bcastVoteChan
}

func (c *bindingCrusaderAgreement) getBcastBindChan() chan byte {
	return c.bcastBindChan
}

func (c *bindingCrusaderAgreement) getOutputExternalValidChan() chan byte {
	return c.outputExternalValidChan
}
