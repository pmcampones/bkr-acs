package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
)

type bindingCrusaderAgreement struct {
	n              uint
	f              uint
	sentEchoes     []bool
	voted          bool
	bound          bool
	echoes         []map[uuid.UUID]bool
	votes          []map[uuid.UUID]bool
	binds          []map[uuid.UUID]bool
	bcastEchoChan  chan byte
	bcastVoteChan  chan byte
	bcastBindChan  chan byte
	outputDecision chan byte
	delivered      bool
	bothChan       chan struct{}
	boundBotChan   chan struct{}
	botChan        chan struct{}
	valChan        chan byte
}

func newBindingCrusaderAgreement(n, f uint) bindingCrusaderAgreement {
	c := bindingCrusaderAgreement{
		n:              n,
		f:              f,
		sentEchoes:     []bool{false, false},
		voted:          false,
		bound:          false,
		echoes:         []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		votes:          []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		binds:          []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		bcastEchoChan:  make(chan byte, 2),
		bcastVoteChan:  make(chan byte, 1),
		bcastBindChan:  make(chan byte, 1),
		outputDecision: make(chan byte, 1),
		delivered:      false,
		bothChan:       make(chan struct{}, 1),
		boundBotChan:   make(chan struct{}, 1),
		botChan:        make(chan struct{}, 1),
		valChan:        make(chan byte, 1),
	}
	go c.tryToOutputBot()
	go c.waitForDecision()
	return c
}

func (c *bindingCrusaderAgreement) propose(est byte) error {
	crusaderLogger.Info("proposing estimate", "est", est)
	if !isInputValid(est) {
		return fmt.Errorf("invalid input %d", est)
	} else if c.sentEchoes[est] {
		crusaderLogger.Debug("already sent echo", "est", est)
	} else {
		c.broadcastEcho(est)
	}
	return nil
}

func (c *bindingCrusaderAgreement) submitEcho(echo byte, sender uuid.UUID) error {
	if !isInputValid(echo) {
		return fmt.Errorf("invalid input %d", echo)
	} else if c.echoes[echo][sender] {
		return fmt.Errorf("duplicate echo from %s", sender)
	}
	crusaderLogger.Debug("submitting echo", "echo", echo, "sender", sender)
	c.echoes[echo][sender] = true
	countEcho := len(c.echoes[echo])
	if countEcho == int(c.f+1) && !c.sentEchoes[echo] {
		c.broadcastEcho(echo)
	} else if countEcho == int(c.n-c.f) {
		countOther := len(c.echoes[1-echo])
		if countOther < int(c.n+c.f) && !c.voted {
			c.broadcastVote(echo)
		} else if !c.bound {
			c.broadcastBind(echo)
		} else if countOther >= int(c.n-c.f) {
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
	crusaderLogger.Debug("submitting vote", "vote", vote, "sender", sender)
	c.votes[vote][sender] = true
	countVotes := len(c.votes[vote])
	if countVotes == int(c.n-c.f) && !c.bound {
		c.broadcastBind(vote)
	}
	return nil
}

func (c *bindingCrusaderAgreement) submitBind(bind byte, sender uuid.UUID) error {
	if !isInputValid(bind) {
		return fmt.Errorf("invalid input %d", bind)
	} else if c.binds[0][sender] || c.binds[1][sender] {
		return fmt.Errorf("duplicate bind from %s", sender)
	}
	crusaderLogger.Debug("submitting bind", "bind", bind, "sender", sender)
	c.binds[bind][sender] = true
	if !c.delivered {
		if len(c.binds[bind]) == int(c.n-c.f) {
			c.delivered = true
			c.valChan <- bind
		} else if len(c.binds[0])+len(c.binds[1]) == int(c.n-c.f) {
			c.delivered = true
			c.boundBotChan <- struct{}{}
		}
	}
	return nil
}

func (c *bindingCrusaderAgreement) tryToOutputBot() {
	<-c.bothChan
	<-c.boundBotChan
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
	c.outputDecision <- decision
}

func (c *bindingCrusaderAgreement) broadcastEcho(echo byte) {
	crusaderLogger.Info("broadcasting echo", "echo", echo)
	c.sentEchoes[echo] = true
	c.bcastEchoChan <- echo
}

func (c *bindingCrusaderAgreement) broadcastVote(vote byte) {
	roundLogger.Info("submitting vote", "vote", vote)
	c.voted = true
	c.bcastVoteChan <- vote
}

func (c *bindingCrusaderAgreement) broadcastBind(bind byte) {
	roundLogger.Info("submitting bind", "bind", bind)
	c.bound = true
	c.bcastBindChan <- bind
}
