package brb

import (
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var instanceLogger = utils.GetLogger(slog.LevelDebug)

// brbHandler defines the functionalities required for handling the brb messages depending on the current phase of the algorithm.
// This implementation follows the State pattern.
type brbHandler interface {
	handleSend(msg []byte)
	handleEcho(msg []byte, id UUID) error
	handleReady(msg []byte, id UUID) error
}

type brbExecutor struct {
	instance  *brbInstance
	commands  chan<- func() error
	closeChan chan<- struct{}
}

func newBrbExecutor(instance *brbInstance) *brbExecutor {
	commands := make(chan func() error)
	closeChan := make(chan struct{})
	executor := &brbExecutor{
		instance:  instance,
		commands:  commands,
		closeChan: closeChan,
	}
	go executor.invoker(commands, closeChan)
	return executor
}

func (e *brbExecutor) handleSend(msg []byte) {
	instanceLogger.Debug("submitting send message handling command")
	e.commands <- func() error {
		e.instance.handleSend(msg)
		return nil
	}
}

func (e *brbExecutor) handleEcho(msg []byte, sender UUID) {
	instanceLogger.Debug("submitting echo message handling command")
	e.commands <- func() error {
		err := e.instance.handleEcho(msg, sender)
		if err != nil {
			return fmt.Errorf("unable to handle echo: %v", err)
		}
		return nil
	}
}

func (e *brbExecutor) handleReady(msg []byte, sender UUID) {
	instanceLogger.Debug("submitting ready message handling command")
	e.commands <- func() error {
		err := e.instance.handleReady(msg, sender)
		if err != nil {
			return fmt.Errorf("unable to handle ready: %v", err)
		}
		return nil
	}
}

func (e *brbExecutor) invoker(commands <-chan func() error, closeChan <-chan struct{}) {
	for {
		select {
		case command := <-commands:
			err := command()
			if err != nil {
				instanceLogger.Error("unable to compute command", "error", err)
			}
		case <-closeChan:
			instanceLogger.Info("closing executor")
			return
		}
	}
}

func (e *brbExecutor) close() {
	instanceLogger.Info("sending signal to close bcb instance")
	e.closeChan <- struct{}{}
}

type brbInstance struct {
	data         *brbData
	peersEchoed  map[UUID]bool
	peersReadied map[UUID]bool
	commands     chan<- func() error
	closeChan    chan<- struct{}
	handler      brbHandler
}

type brbData struct {
	n       uint
	f       uint
	echoes  map[UUID]uint
	readies map[UUID]uint
}

func newBrbInstance(n, f uint, echo, ready, output chan []byte) *brbInstance {
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
		data:         &data,
		peersEchoed:  make(map[UUID]bool),
		peersReadied: make(map[UUID]bool),
		commands:     commands,
		closeChan:    closeChan,
		handler:      ph1,
	}
	go instance.invoker(commands, closeChan)
	return instance
}

func (b *brbInstance) handleSend(msg []byte) {
	instanceLogger.Debug("submitting send message handling command")
	b.commands <- func() error {
		b.handler.handleSend(msg)
		return nil
	}
}

func (b *brbInstance) handleEcho(msg []byte, sender UUID) error {
	instanceLogger.Debug("submitting echo message handling command")
	b.commands <- func() error {
		ok := b.peersEchoed[sender]
		if ok {
			return fmt.Errorf("already received echo from peer %s", sender)
		}
		mid := utils.BytesToUUID(msg)
		b.data.echoes[mid]++
		b.peersEchoed[sender] = true
		err := b.handler.handleEcho(msg, mid)
		if err != nil {
			return fmt.Errorf("unable to handle echo: %v", err)
		}
		return nil
	}
	return nil
}

func (b *brbInstance) handleReady(msg []byte, sender UUID) error {
	instanceLogger.Debug("submitting ready message handling command")
	b.commands <- func() error {
		ok := b.peersReadied[sender]
		if ok {
			return fmt.Errorf("already received ready from peer %s", sender)
		}
		mid := utils.BytesToUUID(msg)
		b.data.readies[mid]++
		b.peersReadied[sender] = true
		err := b.handler.handleReady(msg, mid)
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
				instanceLogger.Error("unable to compute command", "error", err)
			}
		case <-closeChan:
			instanceLogger.Info("closing executor")
			return
		}
	}
}

func (b *brbInstance) close() {
	instanceLogger.Info("sending signal to close bcb instance")
	b.closeChan <- struct{}{}
}
