package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"github.com/google/uuid"
	"log/slog"
	"time"
)

var concurrentMMRLogger = utils.GetLogger("Concurrent MMR", slog.LevelWarn)

type concurrentMMR struct {
	mmr
	commands  chan func()
	closeChan chan struct{}
}

func newConcurrentMMR(n, f uint) concurrentMMR {
	m := concurrentMMR{
		mmr:       newMMR(n, f),
		commands:  make(chan func()),
		closeChan: make(chan struct{}, 1),
	}
	go m.invoker()
	return m
}

func (m *concurrentMMR) invoker() {
	for {
		select {
		case cmd := <-m.commands:
			cmd()
		case <-m.closeChan:
			abaLogger.Info("closing concurrentMMR")
			m.mmr.close()
			return
		}
	}
}

func (m *concurrentMMR) propose(est byte) error {
	concurrentMMRLogger.Info("scheduling initial proposal estimate", "est", est)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.propose(est, 0)
	}
	return <-errChan
}

func (m *concurrentMMR) submitEcho(echo byte, sender uuid.UUID, r uint16) error {
	concurrentMMRLogger.Debug("scheduling submit echo", "echo", echo, "mmrRound", r, "sender", sender)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.submitEcho(echo, sender, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitVote(vote byte, sender uuid.UUID, r uint16) error {
	concurrentMMRLogger.Debug("scheduling submit vote", "vote", vote, "mmrRound", r)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.submitVote(vote, sender, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitBind(bind byte, sender uuid.UUID, r uint16) error {
	concurrentMMRLogger.Debug("scheduling submit bind", "bind", bind, "mmrRound", r)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.submitBind(bind, sender, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitCoin(coin byte, r uint16) error {
	concurrentMMRLogger.Debug("scheduling submit coin", "coin", coin, "mmrRound", r)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.submitCoin(coin, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitDecision(decision byte, sender uuid.UUID) error {
	concurrentMMRLogger.Debug("submitting decision", "decision", decision, "sender", sender)
	return m.mmr.submitDecision(decision, sender)
}

func (m *concurrentMMR) close() {
	concurrentMMRLogger.Info("signaling close concurrentMMR")
	go func() {
		time.Sleep(10 * time.Second)
		m.closeChan <- struct{}{}
	}()
}
