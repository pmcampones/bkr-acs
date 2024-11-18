package byzantineReliableBroadcast

import (
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	on "pace/overlayNetwork"
	"pace/utils"
)

var channelLogger = utils.GetLogger("BRB Channel", slog.LevelWarn)

type BRBMsg struct {
	Content []byte
	Sender  UUID
}

type BRBChannel struct {
	instances     map[UUID]*brbInstance
	finished      map[UUID]bool
	n             uint
	f             uint
	middleware    *brbMiddleware
	BrbDeliver    chan BRBMsg
	commands      chan<- func() error
	closeCommands chan<- struct{}
	closeDeliver  chan<- struct{}
}

func NewBRBChannel(n, f uint, beb *on.BEBChannel) *BRBChannel {
	commands := make(chan func() error)
	deliverChan := make(chan *msg)
	closeCommands := make(chan struct{}, 1)
	closeDeliver := make(chan struct{}, 1)
	channel := &BRBChannel{
		instances:     make(map[UUID]*brbInstance),
		finished:      make(map[UUID]bool),
		n:             n,
		f:             f,
		middleware:    newBRBMiddleware(beb, deliverChan),
		BrbDeliver:    make(chan BRBMsg),
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
		go func() {
			err := instance.send(msg.content, msg.sender)
			if err != nil {
				channelLogger.Warn("unable to process send message", "id", id, "err", err)
			}
		}()
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
	outputChan := make(chan BRBMsg)
	instance := newBrbInstance(c.n, c.f, echoChan, readyChan, outputChan)
	c.instances[id] = instance
	go c.processOutput(outputChan, id)
	return instance
}

func (c *BRBChannel) processOutput(outputChan <-chan BRBMsg, id UUID) {
	output := <-outputChan
	channelLogger.Debug("delivering output message", "id", id)
	c.commands <- func() error {
		go func() { c.BrbDeliver <- output }()
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
		case cmd := <-commands:
			if err := cmd(); err != nil {
				channelLogger.Error("error executing command", "error", err)
			}
		case <-closeCommands:
			channelLogger.Info("closing executor")
			return
		}
	}
}
