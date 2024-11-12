package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"sync/atomic"
)

type concurrentMMR struct {
	handler   *mmr
	commands  chan func()
	closeChan chan struct{}
	isClosed  atomic.Bool
}

func newConcurrentMMR(n, f uint, deliverBVal, deliverAux chan roundMsg, deliverDecision chan byte, coinReq chan uint16) *concurrentMMR {
	m := &concurrentMMR{
		handler:   newMMR(n, f, deliverBVal, deliverAux, deliverDecision, coinReq),
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
		abaLogger.Info("received proposal on closed concurrentMMR")
		return nil
	}
	abaLogger.Info("scheduling initial proposal estimate", "est", est)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.handler.propose(est, firstRound)
	}
	return <-errChan
}

func (m *concurrentMMR) submitBVal(bVal byte, sender uuid.UUID, r uint16) error {
	if m.isClosed.Load() {
		abaLogger.Info("received bVal on closed concurrentMMR")
		return nil
	}
	abaLogger.Debug("scheduling submit bVal", "bVal", bVal, "mmrRound", r, "sender", sender)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.handler.submitBVal(bVal, sender, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitAux(aux byte, sender uuid.UUID, r uint16) error {
	if m.isClosed.Load() {
		abaLogger.Info("received aux on closed concurrentMMR")
		return nil
	}
	abaLogger.Debug("scheduling submit aux", "aux", aux, "mmrRound", r)
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
	abaLogger.Debug("scheduling submit coin", "coin", coin, "mmrRound", r)
	errChan := make(chan error)
	m.commands <- func() {
		errChan <- m.handler.submitCoin(coin, r)
	}
	return <-errChan
}

func (m *concurrentMMR) submitDecision(decision byte, sender uuid.UUID) (byte, error) {
	if m.isClosed.Load() {
		abaLogger.Info("received decision on closed concurrentMMR")
		return bot, nil
	}
	abaLogger.Debug("submitting decision", "decision", decision, "sender", sender)
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
	abaLogger.Info("signaling close concurrentMMR")
	m.closeChan <- struct{}{}
}
