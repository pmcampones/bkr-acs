package coinTosser

import (
	"fmt"
	"github.com/cloudflare/circl/group"
	_ "github.com/cloudflare/circl/group"
	. "github.com/google/uuid"
	"github.com/samber/mo"
	"log/slog"
	"pace/overlayNetwork"
	"pace/utils"
)

var channelLogger = utils.GetLogger(slog.LevelDebug)

type CoinObserver interface {
	DeliverCoin(id UUID, toss bool)
}

type CTChannel struct {
	instances      map[UUID]*coinToss
	finished       map[UUID]bool
	outputChannels map[UUID]chan mo.Result[bool]
	unordered      map[UUID][]func() error
	t              uint
	deal           *deal
	middleware     *ctMiddleware
	commands       chan func() error
	closeCommands  chan struct{}
	closeDeliver   chan struct{}
}

func NewCoinTosserChannel(ssChan *overlayNetwork.SSChannel, bebChan *overlayNetwork.BEBChannel, t uint) (*CTChannel, error) {
	deliverChan := make(chan *msg)
	c := &CTChannel{
		instances:      make(map[UUID]*coinToss),
		outputChannels: make(map[UUID]chan mo.Result[bool]),
		unordered:      make(map[UUID][]func() error),
		finished:       make(map[UUID]bool),
		t:              t,
		middleware:     newCTMiddleware(bebChan, deliverChan),
		commands:       make(chan func() error),
		closeCommands:  make(chan struct{}),
		closeDeliver:   make(chan struct{}),
	}
	go c.initializeChannel(ssChan)
	go c.bebDeliver(deliverChan)
	return c, nil
}

func (c *CTChannel) initializeChannel(ssChan *overlayNetwork.SSChannel) {
	d, err := listenDeal(ssChan.GetSSChan())
	if err != nil {
		channelLogger.Error("unable to listen deal", "error", err)
		return
	}
	c.deal = d
	channelLogger.Info("received deal. Starting to process commands")
	c.invoker()
}

func (c *CTChannel) TossCoin(seed []byte, outputChan chan bool) {
	c.commands <- func() error {
		id := utils.BytesToUUID(seed)
		base := group.Ristretto255.HashToElement(seed, []byte("coin_toss"))
		ct := newCoinToss(c.t, base, c.deal, outputChan)
		c.instances[id] = ct
		channelLogger.Debug("tossing coin", "id", id)
		share, err := ct.tossCoin()
		if err != nil {
			return fmt.Errorf("unable to create toss coin share: %v", err)
		}
		go func() {
			err = c.middleware.broadcastCTShare(id, share)
			if err != nil {
				ctLogger.Error("unable to broadcast coin toss share", "error", err)
			}
		}()
		c.processUnordered(id)
		return nil
	}
}

func (c *CTChannel) processUnordered(id UUID) {
	for _, command := range c.unordered[id] {
		err := command()
		if err != nil {
			outputChan := c.outputChannels[id]
			go func() {
				outputChan <- mo.Err[bool](err)
			}()
		}
	}
	delete(c.unordered, id)
}

func (c *CTChannel) bebDeliver(deliverChan <-chan *msg) {
	for {
		select {
		case msg := <-deliverChan:
			command := func() error { return c.submitShare(msg.id, msg.sender, msg.share) }
			c.scheduleShareSubmission(msg.id, command)
		case <-c.closeDeliver:
			channelLogger.Info("closing deliver executor")
			return
		}
	}
}

func (c *CTChannel) submitShare(id, senderId UUID, ctShare ctShare) error {
	if c.finished[id] {
		return nil
	} else if ct := c.instances[id]; ct == nil {
		return fmt.Errorf("coin toss instance not found")
	} else if err := ct.submitShare(ctShare, senderId); err != nil {
		return fmt.Errorf("unable to submit share: %v", err)
	}
	channelLogger.Debug("submitted share", "id", id, "sender", senderId)
	return nil
}

func (c *CTChannel) scheduleShareSubmission(id UUID, command func() error) {
	c.commands <- func() error {
		if c.finished[id] {
			ctLogger.Debug("ignoring share submission for finished instance", "id", id)
			return nil
		} else if c.instances[id] == nil {
			ctLogger.Debug("scheduling share submission for uninitialized instance", "id", id)
			if c.unordered[id] == nil {
				c.unordered[id] = make([]func() error, 0)
			}
			c.unordered[id] = append(c.unordered[id], command)
		} else {
			ctLogger.Debug("submitting share for initialized instance", "id", id)
			return command()
		}
		return nil
	}
}

func (c *CTChannel) invoker() {
	for {
		select {
		case command := <-c.commands:
			err := command()
			if err != nil {
				channelLogger.Error("error executing command", "error", err)
			}
		case <-c.closeCommands:
			channelLogger.Info("closing executor")
			return
		}
	}
}

func (c *CTChannel) Close() {
	channelLogger.Info("signaling close deliver executor")
	c.closeDeliver <- struct{}{}
	channelLogger.Info("signaling close commands executor")
	c.closeCommands <- struct{}{}
}
