package brb

import (
	"fmt"
	"github.com/google/uuid"
)

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

func (b *brbPhase1Handler) handleSend(msg []byte) error {
	if b.isFinished {
		return b.nextPhase.handleSend(msg)
	} else {
		instanceLogger.Debug("processing send message on phase 1")
		b.isFinished = true
		b.sendEcho(msg)
		return nil
	}
}

func (b *brbPhase1Handler) handleEcho(msg []byte, id uuid.UUID) error {
	if b.isFinished {
		return b.nextPhase.handleEcho(msg, id)
	} else {
		instanceLogger.Debug("processing echo message on phase 1")
		numEchoes, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find echoes in phase 1 with message id %s", id)
		}
		if numEchoes == b.data.f+1 {
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
		instanceLogger.Debug("processing ready message on phase 1")
		numReadies, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find readies with message id %s", id)
		}
		if numReadies == b.data.f+1 {
			b.isFinished = true
			b.sendEcho(msg)
			return b.nextPhase.handleReady(msg, id)
		}
	}
	return nil
}

func (b *brbPhase1Handler) sendEcho(msg []byte) {
	instanceLogger.Info("sending echo message")
	go func() { b.echoChan <- msg }()
}
