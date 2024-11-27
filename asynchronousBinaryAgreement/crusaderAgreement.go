package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var crusaderLogger = utils.GetLogger("Crusader Agreement", slog.LevelWarn)

type crusaderAgreement struct {
	n               uint
	f               uint
	sentEchoes      []bool
	echoes          []map[uuid.UUID]bool
	votes           []map[uuid.UUID]bool
	bcastEchoChan   chan byte
	bcastVoteChan   chan byte
	outputDecision  chan byte
	deliveredSingle bool
}

func newCrusaderAgreement(n, f uint) crusaderAgreement {
	return crusaderAgreement{
		n:               n,
		f:               f,
		sentEchoes:      []bool{false, false},
		echoes:          []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		votes:           []map[uuid.UUID]bool{make(map[uuid.UUID]bool, n), make(map[uuid.UUID]bool, n)},
		bcastEchoChan:   make(chan byte, 2),
		bcastVoteChan:   make(chan byte, 1),
		outputDecision:  make(chan byte, 2),
		deliveredSingle: false,
	}
}

func (c *crusaderAgreement) propose(est byte) error {
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

func (c *crusaderAgreement) submitEcho(echo byte, sender uuid.UUID) error {
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
		if countOther < int(c.n+c.f) {
			c.broadcastVote(echo)
		} else if !c.deliveredSingle {
			c.outputDecision <- bot
		}
	}
	return nil
}

func (c *crusaderAgreement) submitVote(vote byte, sender uuid.UUID) error {
	if !isInputValid(vote) {
		return fmt.Errorf("invalid input %d", vote)
	} else if c.votes[0][sender] || c.votes[1][sender] {
		return fmt.Errorf("duplicate vote from %s", sender)
	}
	crusaderLogger.Debug("submitting vote", "vote", vote, "sender", sender)
	c.votes[vote][sender] = true
	countVotes := len(c.votes[vote])
	if countVotes == int(c.n-c.f) && !c.deliveredSingle {
		c.outputDecision <- vote
	}
	return nil
}

func (c *crusaderAgreement) broadcastEcho(echo byte) {
	crusaderLogger.Info("broadcasting echo", "echo", echo)
	c.sentEchoes[echo] = true
	c.bcastEchoChan <- echo
}

func (c *crusaderAgreement) broadcastVote(vote byte) {
	roundLogger.Info("submitting vote", "vote", vote)
	c.bcastVoteChan <- vote
}
