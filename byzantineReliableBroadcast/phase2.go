package byzantineReliableBroadcast

import (
	"fmt"
	"github.com/google/uuid"
)

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
		instanceLogger.Debug("processing echo message on phase 2")
		numEchoes, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find echoes in phase 2 with message id %s", id)
		}
		if numEchoes == b.data.n-b.data.f {
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
		instanceLogger.Debug("processing ready message on phase 2")
		numReadies, ok := b.data.readies[id]
		if !ok {
			return fmt.Errorf("unable to find readies in phase 2 with message id %s", id)
		}
		if numReadies == b.data.f+1 {
			b.isFinished = true
			b.sendReady(msg)
			return b.nextPhase.handleReady(msg, id)
		}
	}
	return nil
}

func (b *brbPhase2Handler) sendReady(msg []byte) {
	instanceLogger.Info("sending ready message")
	go func() { b.readyChan <- msg }()
}
