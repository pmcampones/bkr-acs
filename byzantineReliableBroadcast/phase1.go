package byzantineReliableBroadcast

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var phase1Logger = utils.GetLogger("BRB Phase 1", slog.LevelWarn)

type brbPhase1Handler struct {
	data       *brbData
	echoChan   chan<- []byte
	isFinished bool
	nextPhase  *brbPhase2Handler
}

func newPhase1Handler(data *brbData, echoChan chan<- []byte, nextPhase *brbPhase2Handler) *brbPhase1Handler {
	return &brbPhase1Handler{
		data:       data,
		echoChan:   echoChan,
		isFinished: false,
		nextPhase:  nextPhase,
	}
}

func (b *brbPhase1Handler) handleSend(msg []byte, sender uuid.UUID) error {
	if !b.isFinished {
		phase1Logger.Debug("processing send message", "sender", sender, "msg", string(msg))
		b.isFinished = true
		senderBytes, err := sender.MarshalBinary()
		if err != nil {
			return fmt.Errorf("unable to marshal sender uuid: %v", err)
		}
		b.sendEcho(append(senderBytes, msg...))
	}
	return nil
}

func (b *brbPhase1Handler) handleEcho(msg []byte, id uuid.UUID) error {
	if b.isFinished {
		return b.nextPhase.handleEcho(msg, id)
	} else {
		numEchoes, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find echoes in phase 1 with message id %s", id)
		}
		phase1Logger.Debug("processing echo message", "sender", id, "msg", string(msg), "received", numEchoes, "required", b.data.f+1)
		if numEchoes == b.data.f+1 {
			phase1Logger.Info("received enough echoes to advance to phase 2")
			b.isFinished = true
			b.sendEcho(msg)
			return b.nextPhase.handleEcho(msg, id)
		}
	}
	return nil
}

func (b *brbPhase1Handler) handleReady(msg []byte, id uuid.UUID) error {
	if b.isFinished {
		return b.nextPhase.handleReady(msg, id)
	} else {
		numReadies, ok := b.data.readies[id]
		if !ok {
			return fmt.Errorf("unable to find readies with message id %s", id)
		}
		instanceLogger.Debug("processing ready message", "sender", id, "msg", string(msg), "received", numReadies, "required", b.data.f+1)
		if numReadies == b.data.f+1 {
			phase1Logger.Info("received enough readies to advance to phase 2")
			b.isFinished = true
			b.sendEcho(msg)
			return b.nextPhase.handleReady(msg, id)
		}
	}
	return nil
}

func (b *brbPhase1Handler) sendEcho(msg []byte) {
	phase1Logger.Info("sending echo message", "msg", string(msg))
	go func() { b.echoChan <- msg }()
}
