package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	brb "pace/byzantineReliableBroadcast"
	ct "pace/coinTosser"
	on "pace/overlayNetwork"
	"pace/utils"
)

var abaChannelLogger = utils.GetLogger(slog.LevelDebug)

type abaWrapper struct {
	instance *abaNetworkedInstance
	output   chan byte
}

type AbaChannel struct {
	n          uint
	f          uint
	instances  map[uuid.UUID]*abaWrapper
	finished   map[uuid.UUID]bool
	ctChannel  *ct.CTChannel
	termidware *terminationMiddleware
	middleware *abaMiddleware
	commands   chan func() error
	closeChan  chan struct{}
}

func NewAbaChannel(n, f uint, dealSS *on.SSChannel, ctBeb, mBeb *on.BEBChannel, tBrb *brb.BRBChannel, t uint) (*AbaChannel, error) {
	ctChannel, err := ct.NewCoinTosserChannel(dealSS, ctBeb, t)
	if err != nil {
		return nil, fmt.Errorf("unable to create coin tosser channel: %w", err)
	}
	c := &AbaChannel{
		n:          n,
		f:          f,
		instances:  make(map[uuid.UUID]*abaWrapper),
		finished:   make(map[uuid.UUID]bool),
		ctChannel:  ctChannel,
		termidware: newTerminationMiddleware(tBrb),
		middleware: newABAMiddleware(mBeb),
		commands:   make(chan func() error),
		closeChan:  make(chan struct{}),
	}
	go c.invoker()
	return c, nil
}

func (c *AbaChannel) listener() {
	for {
		select {
		case term := <-c.termidware.output:
			c.commands <- func() error {
				return c.processTermMsg(term)
			}
		case abamsg := <-c.middleware.output:
			c.commands <- func() error {
				return c.processMiddlewareMsg(abamsg)
			}
		}
	}
}

func (c *AbaChannel) processTermMsg(term *terminationMsg) error {
	wrapper, err := c.getInstance(term.instance)
	if err != nil {
		return fmt.Errorf("unable to get aba instance: %w", err)
	}
	go wrapper.instance.submitDecision(term.decision, term.sender)
	return nil
}

func (c *AbaChannel) processMiddlewareMsg(msg *abaMsg) error {
	wrapper, err := c.getInstance(msg.instance)
	if err != nil {
		return fmt.Errorf("unable to get aba instance: %w", err)
	}
	switch msg.kind {
	case bval:
		go wrapper.instance.submitBVal(msg.val, msg.sender, msg.round)
	case aux:
		go wrapper.instance.submitAux(msg.val, msg.sender, msg.round)
	}
	return nil
}

func (c *AbaChannel) getInstance(id uuid.UUID) (*abaWrapper, error) {
	if c.finished[id] {
		return nil, fmt.Errorf("requested instance is already finished")
	}
	instance := c.instances[id]
	if instance == nil {
		instance = c.newWrappedInstance(id)
	}
	return instance, nil
}

func (c *AbaChannel) newWrappedInstance(id uuid.UUID) *abaWrapper {
	abaNetworked := newAbaNetworkedInstance(id, c.n, c.f, c.middleware, c.termidware, c.ctChannel)
	wrapper := &abaWrapper{
		instance: abaNetworked,
		output:   make(chan byte, 1),
	}
	c.instances[id] = wrapper
	go c.handleAsyncResultDelivery(id, wrapper)
	return wrapper
}

func (c *AbaChannel) handleAsyncResultDelivery(id uuid.UUID, wrapper *abaWrapper) {
	finalDecision := <-wrapper.instance.output
	wrapper.output <- finalDecision
	c.commands <- func() error {
		return c.closeWrappedInstance(id)
	}
}

func (c *AbaChannel) closeWrappedInstance(id uuid.UUID) error {
	if c.finished[id] {
		return fmt.Errorf("instance already closed")
	} else if instance := c.instances[id]; instance == nil {
		return fmt.Errorf("instance does not exist")
	} else {
		c.finished[id] = true
		c.instances[id] = nil
		instance.instance.close()
	}
	return nil
}

func (c *AbaChannel) invoker() {
	for {
		select {
		case cmd := <-c.commands:
			if err := cmd(); err != nil {
				abaChannelLogger.Warn("error executing command", "error", err)
			}
		case <-c.closeChan:
			abaChannelLogger.Info("closing asynchronousBinaryAgreement channel")
			return
		}
	}
}

func (c *AbaChannel) propose(instanceId uuid.UUID, val byte, output chan byte) {
	c.commands <- func() error {
		abaChannelLogger.Debug("submitting initial proposal", "instanceId", instanceId, "val", val)
		if c.finished[instanceId] {
			return fmt.Errorf("instance already finished")
		} else if c.instances[instanceId] != nil {
			return fmt.Errorf("instance already exists")
		}

		return nil
	}
}

func (c *AbaChannel) close() {
	abaChannelLogger.Info("signaling close of asynchronousBinaryAgreement channel")
	c.closeChan <- struct{}{}
}
