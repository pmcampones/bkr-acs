package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var termLocalLogger = utils.GetLogger("Local MMR Termination", slog.LevelWarn)

const bot byte = 2

type termOutput struct {
	decision byte
	err      error
}

type mmrTermination struct {
	received  map[uuid.UUID]bool
	results   []uint
	f         uint
	commands  chan func()
	closeChan chan struct{}
}

func newMmrTermination(f uint) *mmrTermination {
	t := &mmrTermination{
		received:  make(map[uuid.UUID]bool),
		results:   []uint{0, 0},
		f:         f,
		commands:  make(chan func()),
		closeChan: make(chan struct{}, 1),
	}
	go t.invoker()
	termLocalLogger.Info("new mmrTermination created")
	return t
}

func (t *mmrTermination) invoker() {
	for {
		select {
		case cmd := <-t.commands:
			cmd()
		case <-t.closeChan:
			abaLogger.Info("closing mmrTermination")
			return
		}
	}
}

func (t *mmrTermination) submitDecision(decision byte, sender uuid.UUID) (byte, error) {
	output := make(chan termOutput, 1)
	t.commands <- func() {
		res := termOutput{
			decision: bot,
			err:      nil,
		}
		if t.received[sender] {
			res.err = fmt.Errorf("sender %s already submitted a decision", sender)
		} else if decision >= bot {
			res.err = fmt.Errorf("invalid decision %d", decision)
		} else {
			t.received[sender] = true
			t.results[decision]++
			abaLogger.Debug("submitting decision", "decision", decision, "sender", sender, "received", t.results[decision], "required", t.f+1)
			if t.results[decision] == t.f+1 {
				abaLogger.Info("decision reached", "decision", decision)
				res.decision = decision
			}
		}
		output <- res
	}
	res := <-output
	return res.decision, res.err
}

func (t *mmrTermination) close() {
	abaLogger.Info("signaling close mmrTermination")
	t.closeChan <- struct{}{}
}
