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

var abaChannelLogger = utils.GetLogger(slog.LevelWarn)

type abaWrapper struct {
	instance *abaNetworkedInstance
	output   chan byte
}

type AbaChannel struct {
	n             uint
	f             uint
	instances     map[uuid.UUID]*abaWrapper
	finished      map[uuid.UUID]bool
	ctChannel     *ct.CTChannel
	termidware    *terminationMiddleware
	middleware    *abaMiddleware
	commands      chan func() error
	listenerClose chan struct{}
	invokerClose  chan struct{}
}

func NewAbaChannel(n, f uint, dealSS *on.SSChannel, ctBeb, mBeb *on.BEBChannel, tBrb *brb.BRBChannel) (*AbaChannel, error) {
	ctChannel, err := ct.NewCoinTosserChannel(dealSS, ctBeb, f)
	if err != nil {
		return nil, fmt.Errorf("unable to create coin tosser channel: %w", err)
	}
	c := &AbaChannel{
		n:             n,
		f:             f,
		instances:     make(map[uuid.UUID]*abaWrapper),
		finished:      make(map[uuid.UUID]bool),
		ctChannel:     ctChannel,
		termidware:    newTerminationMiddleware(tBrb),
		middleware:    newABAMiddleware(mBeb),
		commands:      make(chan func() error),
		listenerClose: make(chan struct{}, 1),
		invokerClose:  make(chan struct{}, 1),
	}
	go c.invoker()
	go c.listener()
	return c, nil
}

func (c *AbaChannel) NewAbaInstance(instanceId uuid.UUID) chan byte {
	res := make(chan chan byte, 1)
	c.commands <- func() error {
		if _, err := c.getInstance(instanceId); err != nil {
			return fmt.Errorf("unable to get aba instance: %w", err)
		}
		res <- c.instances[instanceId].output
		return nil
	}
	return <-res
}

func (c *AbaChannel) Propose(instanceId uuid.UUID, val byte) {
	c.commands <- func() error {
		if wrapper, err := c.getInstance(instanceId); err != nil {
			return fmt.Errorf("unable to get aba instance: %w", err)
		} else if err = wrapper.instance.propose(val); err != nil {
			return fmt.Errorf("unable to propose value in instance %s: %w ", instanceId, err)
		}
		return nil
	}
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
		case <-c.listenerClose:
			abaChannelLogger.Info("closing listener")
			return
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
		case <-c.invokerClose:
			abaChannelLogger.Info("closing asynchronousBinaryAgreement channel")
			return
		}
	}
}

func (c *AbaChannel) Close() {
	abaChannelLogger.Info("signaling close of asynchronousBinaryAgreement channel")
	c.listenerClose <- struct{}{}
	c.invokerClose <- struct{}{}
}
