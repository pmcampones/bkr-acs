package byzantineReliableBroadcast

import (
	"crypto/ecdsa"
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var logger = utils.GetLogger(slog.LevelWarn)

// brbState defines the functionalities required for handling the brb messages depending on the current phase of the algorithm.
// This implementation follows the State pattern.
type brbState interface {
	handleSend(msg []byte) error
	handleEcho(msg []byte, id UUID) error
	handleReady(msg []byte, id UUID) error
}

type brbInstance struct {
	data          *brbData
	peersEchoed   map[UUID]bool
	peersReadied  map[UUID]bool
	commands      chan<- func() error
	closeChan     chan<- struct{}
	concreteState brbState
}

type brbData struct {
	n       uint
	f       uint
	echoes  map[UUID]uint
	readies map[UUID]uint
}

func newBrbInstance(n, f uint, echo, ready, output chan []byte) (*brbInstance, error) {
	data := brbData{
		n:       n,
		f:       f,
		echoes:  make(map[UUID]uint),
		readies: make(map[UUID]uint),
	}
	ph3 := newPhase3Handler(&data, output)
	ph2 := newPhase2Handler(&data, ready, ph3)
	ph1 := newPhase1Handler(&data, echo, ph2)
	commands := make(chan func() error)
	closeChan := make(chan struct{})
	instance := &brbInstance{
		data:          &data,
		peersEchoed:   make(map[UUID]bool),
		peersReadied:  make(map[UUID]bool),
		commands:      commands,
		closeChan:     closeChan,
		concreteState: ph1,
	}
	go instance.invoker(commands, closeChan)
	return instance, nil
}

func (b *brbInstance) handleSend(msg []byte) {
	logger.Debug("submitting send message handling command")
	b.commands <- func() error {
		err := b.concreteState.handleSend(msg)
		if err != nil {
			return fmt.Errorf("unable to handle send: %v", err)
		}
		return nil
	}
}

func (b *brbInstance) handleEcho(msg []byte, id UUID, sender *ecdsa.PublicKey) error {
	logger.Debug("submitting echo message handling command")
	senderId, err := utils.PkToUUID(sender)
	if err != nil {
		return fmt.Errorf("unable to get sender id: %v", err)
	}
	b.commands <- func() error {
		ok := b.peersEchoed[senderId]
		if ok {
			return fmt.Errorf("already received echo from peer %s", *sender)
		}
		b.data.echoes[id]++
		b.peersEchoed[senderId] = true
		err := b.concreteState.handleEcho(msg, id)
		if err != nil {
			return fmt.Errorf("unable to handle echo: %v", err)
		}
		return nil
	}
	return nil
}

func (b *brbInstance) handleReady(msg []byte, id UUID, sender *ecdsa.PublicKey) error {
	logger.Debug("submitting ready message handling command")
	senderId, err := utils.PkToUUID(sender)
	if err != nil {
		return fmt.Errorf("unable to get sender id: %v", err)
	}
	b.commands <- func() error {
		ok := b.peersReadied[senderId]
		if ok {
			return fmt.Errorf("already received ready from peer %s", *sender)
		}
		b.data.readies[id]++
		b.peersReadied[senderId] = true
		err := b.concreteState.handleReady(msg, id)
		if err != nil {
			return fmt.Errorf("unable to handle ready: %v", err)
		}
		return nil
	}
	return nil
}

func (b *brbInstance) invoker(commands <-chan func() error, closeChan <-chan struct{}) {
	for {
		select {
		case command := <-commands:
			err := command()
			if err != nil {
				logger.Error("unable to compute command", "error", err)
			}
		case <-closeChan:
			logger.Debug("closing byzantineReliableBroadcast executor")
			return
		}
	}
}

func (b *brbInstance) close() {
	logger.Debug("sending signal to close bcb instance")
	b.closeChan <- struct{}{}
}
