package brb

import (
	"crypto/ecdsa"
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	"pace/overlayNetwork"
	"pace/utils"
)

var channelLogger = utils.GetLogger(slog.LevelWarn)

type BRBObserver interface {
	BRBDeliver(msg []byte)
}

type BRBChannel struct {
	instances  map[UUID]*brbInstance
	finished   map[UUID]bool
	n          uint
	f          uint
	observers  []BRBObserver
	middleware *brbMiddleware
	commands   chan<- func() error
	listenCode byte
}

func CreateBRBChannel(n, f uint, node *overlayNetwork.Node, listenCode byte) *BRBChannel {
	commands := make(chan func() error)
	channel := &BRBChannel{
		instances:  make(map[UUID]*brbInstance),
		finished:   make(map[UUID]bool),
		n:          n,
		f:          f,
		middleware: newBRBMiddleware(node, listenCode),
		observers:  make([]BRBObserver, 0),
		commands:   commands,
		listenCode: utils.GetCode("brb_code"),
	}
	node.AttachMessageObserver(channel)
	go invoker(commands)
	return channel
}

func (c *BRBChannel) AttachObserver(observer BRBObserver) {
	channelLogger.Info("attaching observer to bcb channel", "observer", observer)
	c.observers = append(c.observers, observer)
}

func (c *BRBChannel) BRBroadcast(msg []byte) error {
	return c.middleware.broadcastSend(msg)
}

func (c *BRBChannel) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	if msg[0] == c.listenCode {
		structMsg, err := c.middleware.processMsg(msg, sender)
		if err != nil {
			channelLogger.Warn("unable to processMsg message during beb delivery", "error", err)
		}
		c.commands <- func() error { return c.processMsg(structMsg) }
	} else {
		channelLogger.Debug("received message was not for me", "sender", sender, "listenCode", msg[0])
	}
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
		go instance.send(msg.content)
	case echo:
		go func() {
			err := instance.echo(msg.content, msg.sender)
			if err != nil {
				channelLogger.Warn("unable to process echo message", "id", id, "err", err)
			}
		}()
	case ready:
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
		for _, observer := range c.observers {
			observer.BRBDeliver(output)
		}
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

func invoker(commands <-chan func() error) {
	for command := range commands {
		err := command()
		if err != nil {
			channelLogger.Error("error executing command", "error", err)
		}
	}
}
