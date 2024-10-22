package brb

import (
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	"pace/overlayNetwork"
	"pace/utils"
)

var channelLogger = utils.GetLogger(slog.LevelWarn)

type BRBChannel struct {
	instances     map[UUID]*brbInstance
	finished      map[UUID]bool
	n             uint
	f             uint
	middleware    *brbMiddleware
	brbDeliver    chan<- []byte
	commands      chan<- func() error
	closeCommands chan<- struct{}
	closeDeliver  chan<- struct{}
}

func CreateBRBChannel(n, f uint, beb *overlayNetwork.BEBChannel, brbDeliver chan<- []byte) *BRBChannel {
	commands := make(chan func() error)
	deliverChan := make(chan *msg)
	closeCommands := make(chan struct{})
	closeDeliver := make(chan struct{})
	channel := &BRBChannel{
		instances:     make(map[UUID]*brbInstance),
		finished:      make(map[UUID]bool),
		n:             n,
		f:             f,
		middleware:    newBRBMiddleware(beb, deliverChan),
		brbDeliver:    brbDeliver,
		commands:      commands,
		closeCommands: closeCommands,
		closeDeliver:  closeDeliver,
	}
	go invoker(commands, closeCommands)
	go channel.bebDeliver(deliverChan, closeDeliver)
	channelLogger.Info("BRB channel created", "n", n, "f", f)
	return channel
}

func (c *BRBChannel) BRBroadcast(msg []byte) error {
	channelLogger.Debug("broadcasting message", "msg", string(msg))
	return c.middleware.broadcastSend(msg)
}

func (c *BRBChannel) processMsg(msg *msg) error {
	id := msg.id
	if c.finished[id] {
		channelLogger.Debug("received message from finished instance", "id", id)
		return nil
	}
	instance, ok := c.instances[id]
	if !ok {
		instance = c.createInstance(msg.id)
	}
	switch msg.kind {
	case send:
		channelLogger.Debug("processing send message", "id", id, "from", msg.sender, "content", string(msg.content))
		go instance.send(msg.content)
	case echo:
		channelLogger.Debug("processing echo message", "id", id, "from", msg.sender, "content", string(msg.content))
		go func() {
			err := instance.echo(msg.content, msg.sender)
			if err != nil {
				channelLogger.Warn("unable to process echo message", "id", id, "err", err)
			}
		}()
	case ready:
		channelLogger.Debug("processing ready message", "id", id, "from", msg.sender, "content", string(msg.content))
		go func() {
			err := instance.ready(msg.content, msg.sender)
			if err != nil {
				channelLogger.Warn("unable to process ready message", "id", id, "err", err)
			}
		}()
	default:
		return fmt.Errorf("unhandled default case in message processing")
	}
	return nil
}

func (c *BRBChannel) createInstance(id UUID) *brbInstance {
	echoChan, readyChan := c.middleware.makeChannels(id)
	outputChan := make(chan []byte)
	instance := newBrbInstance(c.n, c.f, echoChan, readyChan, outputChan)
	c.instances[id] = instance
	go c.processOutput(outputChan, id)
	return instance
}

func (c *BRBChannel) processOutput(outputChan <-chan []byte, id UUID) {
	output := <-outputChan
	channelLogger.Debug("delivering output message", "id", id)
	c.commands <- func() error {
		go func() { c.brbDeliver <- output }()
		instance, ok := c.instances[id]
		if !ok {
			return fmt.Errorf("channel handler %s not found upon delivery", id)
		}
		c.finished[id] = true
		delete(c.instances, id)
		go instance.close()
		return nil
	}
}

func (c *BRBChannel) Close() {
	c.closeCommands <- struct{}{}
	c.closeDeliver <- struct{}{}
}

func (c *BRBChannel) bebDeliver(deliverChan <-chan *msg, closeDeliver <-chan struct{}) {
	for {
		select {
		case deliver := <-deliverChan:
			c.commands <- func() error {
				return c.processMsg(deliver)
			}
		case <-closeDeliver:
			channelLogger.Info("closing deliver executor")
			return
		}
	}
}

func invoker(commands <-chan func() error, closeCommands <-chan struct{}) {
	for {
		select {
		case command := <-commands:
			err := command()
			if err != nil {
				channelLogger.Error("error executing command", "error", err)
			}
		case <-closeCommands:
			channelLogger.Info("closing executor")
			return
		}
	}
}
