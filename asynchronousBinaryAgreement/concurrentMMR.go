package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
	"sync/atomic"
	"time"
)

var concurrentMMRLogger = utils.GetLogger("Concurrent MMR", slog.LevelDebug)

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

func (m *concurrentMMR) submitBVal(bVal byte, sender uuid.UUID, r uint16) error {
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received bVal on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Debug("scheduling submit bVal", "bVal", bVal, "mmrRound", r, "sender", sender)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.handler.submitBVal(bVal, sender, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitAux(aux byte, sender uuid.UUID, r uint16) error {
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received aux on closed concurrentMMR")
		return nil
	}
	concurrentMMRLogger.Debug("scheduling submit aux", "aux", aux, "mmrRound", r)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.handler.submitAux(aux, sender, r)
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

func (m *concurrentMMR) submitDecision(decision byte, sender uuid.UUID) (byte, error) {
	if m.isClosed.Load() {
		concurrentMMRLogger.Info("received decision on closed concurrentMMR")
		return bot, nil
	}
	concurrentMMRLogger.Debug("submitting decision", "decision", decision, "sender", sender)
	res := make(chan termOutput, 1)
	m.commands <- func() {
		result, err := m.handler.submitDecision(decision, sender)
		res <- termOutput{decision: result, err: err}
	}
	output := <-res
	if output.err != nil {
		return bot, fmt.Errorf("unable to submit decision: %v", output.err)
	}
	return output.decision, nil
}

func (m *concurrentMMR) close() {
	m.isClosed.Store(true)
	concurrentMMRLogger.Info("signaling close concurrentMMR")
	go func() {
		time.Sleep(10 * time.Second)
		m.closeChan <- struct{}{}
	}()
}

func (m *concurrentMMR) getBValChan() chan roundMsg {
	return m.handler.deliverBVal
}

func (m *concurrentMMR) getAuxChan() chan roundMsg {
	return m.handler.deliverAux
}

func (m *concurrentMMR) getDecisionChan() chan byte {
	return m.handler.deliverDecision
}

func (m *concurrentMMR) getCoinReqChan() chan uint16 {
	return m.handler.coinReq
}
