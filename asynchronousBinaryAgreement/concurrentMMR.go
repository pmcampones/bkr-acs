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
	handler   *mmr
	commands  chan func()
	closeChan chan struct{}
	isClosed  atomic.Bool
}

func newConcurrentMMR(n, f uint) *concurrentMMR {
	m := &concurrentMMR{
		handler:   newMMR(n, f),
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
			m.handler.close()
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
		errChan <- m.handler.propose(est, firstRound)
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
		errChan <- m.handler.submitEcho(echo, sender, r)
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
		errChan <- m.handler.submitVote(vote, sender, r)
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
		errChan <- m.handler.submitCoin(coin, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitDecision(decision byte, sender uuid.UUID) error {
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received decision on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Debug("submitting decision", "decision", decision, "sender", sender)
	return m.handler.submitDecision(decision, sender)
}

func (m *concurrentMMR) close() {
	m.isClosed.Store(true)
	concurrentMMRLogger.Info("signaling close concurrentMMR")
	go func() {
		time.Sleep(10 * time.Second)
		m.closeChan <- struct{}{}
	}()
}

func (m *concurrentMMR) getEchoChan() chan roundMsg {
	return m.handler.deliverEcho
}

func (m *concurrentMMR) getVoteChan() chan roundMsg {
	return m.handler.deliverVote
}

func (m *concurrentMMR) getDecisionChan() chan byte {
	return m.handler.deliverDecision
}

func (m *concurrentMMR) getCoinReqChan() chan uint16 {
	return m.handler.coinReq
}
