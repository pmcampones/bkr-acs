package asynchronousBinaryAgreement

import (
	ct "bkr-acs/coinTosser"
	on "bkr-acs/overlayNetwork"
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var abaChannelLogger = utils.GetLogger("ABA Channel", slog.LevelWarn)

type AbaInstance struct {
	inner  *abaNetworkedInstance
	output chan byte
}

func (a *AbaInstance) Propose(est byte) error {
	if err := a.inner.propose(est); err != nil {
		return fmt.Errorf("unable to propose initial estimate: %w", err)
	}
	return nil
}

func (a *AbaInstance) GetOutput() byte {
	return <-a.output
}

type AbaChannel struct {
	n             uint
	f             uint
	instances     map[uuid.UUID]*AbaInstance
	finished      map[uuid.UUID]bool
	ctChannel     *ct.CTChannel
	termidware    *terminationMiddleware
	middleware    *abaMiddleware
	commands      chan func() error
	listenerClose chan struct{}
	invokerClose  chan struct{}
}

func NewAbaChannel(n, f uint, dealSS *on.SSChannel, ctBeb, mBeb, tBeb *on.BEBChannel) (*AbaChannel, error) {
	ctChannel, err := ct.NewCoinTosserChannel(dealSS, ctBeb, f)
	if err != nil {
		return nil, fmt.Errorf("unable to create coin tosser channel: %w", err)
	}
	c := &AbaChannel{
		n:             n,
		f:             f,
		instances:     make(map[uuid.UUID]*AbaInstance),
		finished:      make(map[uuid.UUID]bool),
		ctChannel:     ctChannel,
		termidware:    newTerminationMiddleware(tBeb),
		middleware:    newABAMiddleware(mBeb),
		commands:      make(chan func() error),
		listenerClose: make(chan struct{}, 1),
		invokerClose:  make(chan struct{}, 1),
	}
	go c.invoker()
	go c.listener()
	abaChannelLogger.Info("initialized aba channel", "n", n, "f", f)
	return c, nil
}

func (c *AbaChannel) NewAbaInstance(instanceId uuid.UUID) *AbaInstance {
	res := make(chan *AbaInstance, 1)
	c.commands <- func() error {
		if instance, err := c.getInstance(instanceId); err != nil {
			res <- nil
			return fmt.Errorf("unable to get aba inner: %w", err)
		} else {
			abaChannelLogger.Debug("outputting aba instance", "id", instanceId)
			res <- instance
		}
		return nil
	}
	return <-res
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
	instance, err := c.getInstance(term.instance)
	if err != nil {
		return fmt.Errorf("unable to process termination message: unable to get aba inner: %w", err)
	}
	go func() {
		err := instance.inner.submitDecision(term.decision, term.sender)
		if err != nil {
			abaChannelLogger.Warn("unable to submit decision", "instanceId", term.instance, "decision", term.decision, "error", err)
		}
	}()
	return nil
}

func (c *AbaChannel) processMiddlewareMsg(msg *abaMsg) error {
	wrapper, err := c.getInstance(msg.instance)
	if err != nil {
		return fmt.Errorf("unable to process aba control message: unable to get aba inner: %w", err)
	}
	switch msg.kind {
	case echo:
		go func() {
			err := wrapper.inner.submitEcho(msg.val, msg.sender, msg.round)
			if err != nil {
				abaChannelLogger.Warn("unable to submit bVal", "instanceId", msg.instance, "round", msg.round, "error", err)
			}
		}()
	case vote:
		go func() {
			err := wrapper.inner.submitVote(msg.val, msg.sender, msg.round)
			if err != nil {
				abaChannelLogger.Warn("unable to submit vote", "instanceId", msg.instance, "round", msg.round, "error", err)
			}
		}()
	case bind:
		go func() {
			err := wrapper.inner.submitBind(msg.val, msg.sender, msg.round)
			if err != nil {
				abaChannelLogger.Warn("unable to submit bind", "instanceId", msg.instance, "round", msg.round, "error", err)
			}
		}()
	}
	return nil
}

func (c *AbaChannel) getInstance(id uuid.UUID) (*AbaInstance, error) {
	if c.finished[id] {
		return nil, fmt.Errorf("requested instance is already finished")
	}
	instance := c.instances[id]
	if instance == nil {
		instance = c.newAbaInstance(id)
	}
	return instance, nil
}

func (c *AbaChannel) newAbaInstance(id uuid.UUID) *AbaInstance {
	abaNetworked := newAbaNetworkedInstance(id, c.n, c.f, c.middleware, c.termidware, c.ctChannel)
	wrapper := &AbaInstance{
		inner:  abaNetworked,
		output: make(chan byte, 1),
	}
	c.instances[id] = wrapper
	go c.handleAsyncResultDelivery(id, wrapper)
	abaChannelLogger.Debug("created new aba instance", "id", id)
	return wrapper
}

func (c *AbaChannel) handleAsyncResultDelivery(id uuid.UUID, wrapper *AbaInstance) {
	finalDecision := <-wrapper.inner.decisionChan
	abaChannelLogger.Debug("outputting decision for aba instance", "id", id, "decision", finalDecision)
	wrapper.output <- finalDecision
	<-wrapper.inner.terminatedChan
	abaChannelLogger.Info("closing aba instance", "id", id)
	c.commands <- func() error {
		return c.closeWrappedInstance(id)
	}
}

func (c *AbaChannel) closeWrappedInstance(id uuid.UUID) error {
	if c.finished[id] {
		return fmt.Errorf("inner already closed")
	} else if instance := c.instances[id]; instance == nil {
		return fmt.Errorf("inner does not exist")
	} else {
		c.finished[id] = true
		c.instances[id] = nil
		instance.inner.close()
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
			abaChannelLogger.Info("closing invoker")
			return
		}
	}
}

func (c *AbaChannel) Close() {
	abaChannelLogger.Info("signaling close of listener and invoker")
	c.listenerClose <- struct{}{}
	c.invokerClose <- struct{}{}
}
