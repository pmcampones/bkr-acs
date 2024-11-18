package byzantineReliableBroadcast

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var phase2Logger = utils.GetLogger("BRB Phase 2", slog.LevelWarn)

type brbPhase2Handler struct {
	data       *brbData
	readyChan  chan<- []byte
	isFinished bool
	nextPhase  *brbPhase3Handler
}

func newPhase2Handler(data *brbData, readyChan chan<- []byte, nextPhase *brbPhase3Handler) *brbPhase2Handler {
	return &brbPhase2Handler{
		data:       data,
		readyChan:  readyChan,
		isFinished: false,
		nextPhase:  nextPhase,
	}
}

func (b *brbPhase2Handler) handleEcho(msg []byte, id uuid.UUID) error {
	if b.isFinished {
		return nil
	} else {
		numEchoes, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find echoes in phase 2 with message id %s", id)
		}
		phase2Logger.Debug("processing echo message", "sender", id, "msg", string(msg), "received", numEchoes, "required", b.data.n-b.data.f)
		if numEchoes == b.data.n-b.data.f {
			phase2Logger.Info("received enough echoes to advance to phase 3")
			b.isFinished = true
			b.sendReady(msg)
			return nil
		}
	}
	return nil
}

func (b *brbPhase2Handler) handleReady(msg []byte, id uuid.UUID) error {
	if b.isFinished {
		return b.nextPhase.handleReady(msg, id)
	} else {
		numReadies, ok := b.data.readies[id]
		if !ok {
			return fmt.Errorf("unable to find readies in phase 2 with message id %s", id)
		}
		phase2Logger.Debug("processing ready message", "sender", id, "msg", string(msg), "received", numReadies, "required", b.data.f+1)
		if numReadies == b.data.f+1 {
			phase2Logger.Info("received enough readies to advance to phase 3")
			b.isFinished = true
			b.sendReady(msg)
			return b.nextPhase.handleReady(msg, id)
		}
	}
	return nil
}

func (b *brbPhase2Handler) sendReady(msg []byte) {
	phase2Logger.Info("sending ready message", "msg", string(msg))
	go func() { b.readyChan <- msg }()
}
