package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"github.com/google/uuid"
	"log/slog"
	"sync/atomic"
	"time"
)

var concurrentMMRLogger = utils.GetLogger("Concurrent MMR", slog.LevelWarn)

type concurrentMMR struct {
	mmr
	commands  chan func()
	closeChan chan struct{}
	isClosed  atomic.Bool
}

func newConcurrentMMR(n, f uint) *concurrentMMR {
	m := &concurrentMMR{
		mmr:       newMMR(n, f),
		commands:  make(chan func()),
		closeChan: make(chan struct{}, 1),
		isClosed:  atomic.Bool{},
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
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received proposal on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Info("scheduling initial proposal estimate", "est", est)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.propose(est, firstRound)
	}
	return <-errChan
}

func (m *concurrentMMR) submitEcho(echo byte, sender uuid.UUID, r uint16) error {
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received echo on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Debug("scheduling submit echo", "echo", echo, "mmrRound", r, "sender", sender)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.submitEcho(echo, sender, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitVote(vote byte, sender uuid.UUID, r uint16) error {
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received vote on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Debug("scheduling submit vote", "vote", vote, "mmrRound", r)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.submitVote(vote, sender, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitCoin(coin byte, r uint16) error {
	if m.isClosed.Load() {
		abaLogger.Info("received coin on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Debug("scheduling submit coin", "coin", coin, "mmrRound", r)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.mmr.submitCoin(coin, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitDecision(decision byte, sender uuid.UUID) error {
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received decision on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Debug("submitting decision", "decision", decision, "sender", sender)
	return m.mmr.submitDecision(decision, sender)
}

func (m *concurrentMMR) close() {
	m.isClosed.Store(true)
	concurrentMMRLogger.Info("signaling close concurrentMMR")
	go func() {
		time.Sleep(10 * time.Second)
		m.closeChan <- struct{}{}
	}()
}
