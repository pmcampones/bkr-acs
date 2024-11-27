package asynchronousBinaryAgreement

import (
	ct "bkr-acs/coinTosser"
	"bkr-acs/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"sync"
	"unsafe"
)

var abaNetworkedLogger = utils.GetLogger("ABA Networked Instance", slog.LevelWarn)

type abaNetworkedInstance struct {
	id             uuid.UUID
	instance       *concurrentMMR
	decisionChan   chan byte
	terminatedChan chan struct{}
	hasDelivered   bool
	deliveryLock   sync.Mutex
	abamidware     *abaMiddleware
	termidware     *terminationMiddleware
	ctChan         *ct.CTChannel
	listenerClose  chan struct{}
}

func newAbaNetworkedInstance(id uuid.UUID, n, f uint, abamidware *abaMiddleware, termidware *terminationMiddleware, ctChan *ct.CTChannel) *abaNetworkedInstance {
	a := &abaNetworkedInstance{
		id:             id,
		instance:       newConcurrentMMR(n, f),
		decisionChan:   make(chan byte, 1),
		terminatedChan: make(chan struct{}, 1),
		hasDelivered:   false,
		deliveryLock:   sync.Mutex{},
		abamidware:     abamidware,
		termidware:     termidware,
		ctChan:         ctChan,
		listenerClose:  make(chan struct{}),
	}
	go a.listener()
	return a
}

func (a *abaNetworkedInstance) propose(est byte) error {
	abaNetworkedLogger.Info("proposing initial estimate", "instance", a.id, "est", est)
	if err := a.instance.propose(est); err != nil {
		return fmt.Errorf("unable to propose initial estimate: %w", err)
	}
	return nil
}

func (a *abaNetworkedInstance) listener() {
	abaNetworkedLogger.Info("starting listener aba networked inner", "instance", a.id)
	for {
		select {
		case echo := <-a.instance.getEchoChan():
			if err := a.abamidware.broadcastBVal(a.id, echo.r, echo.val); err != nil {
				abaNetworkedLogger.Warn("unable to broadcast echo", "instance", a.id, "round", echo.r, "error", err)
			}
		case vote := <-a.instance.getVoteChan():
			if err := a.abamidware.broadcastAux(a.id, vote.r, vote.val); err != nil {
				abaNetworkedLogger.Warn("unable to broadcast vote", "instance", a.id, "round", vote.r, "error", err)
			}
		case decision := <-a.instance.getDecisionChan():
			if a.canOutputDecision() {
				abaNetworkedLogger.Info("outputting decision", "instance", a.id, "decision", decision)
				a.outputDecision(decision)
			}
		case coinReq := <-a.instance.getCoinReqChan():
			go func() {
				coin, err := a.getCoin(coinReq)
				if err != nil {
					abaNetworkedLogger.Warn("unable to get coin", "instance", a.id, "round", coinReq, "error", err)
				} else if err := a.instance.submitCoin(coin, coinReq); err != nil {
					abaNetworkedLogger.Warn("unable to submit coin", "instance", a.id, "round", coinReq, "error", err)
				}
			}()
		case <-a.listenerClose:
			abaNetworkedLogger.Info("closing listener", "instance", a.id)
			return
		}
	}
}

func (a *abaNetworkedInstance) getCoin(round uint16) (byte, error) {
	coinReqSeed, err := a.makeCoinSeed(round)
	if err != nil {
		return bot, fmt.Errorf("unable to make coin seed: %w", err)
	}
	coinReceiver := make(chan bool)
	abaNetworkedLogger.Debug("requesting coin", "instance", a.id, "round", round)
	a.ctChan.TossCoin(coinReqSeed, coinReceiver)
	coin := <-coinReceiver
	if coin {
		return 1, nil
	} else {
		return 0, nil
	}
}

func (a *abaNetworkedInstance) makeCoinSeed(round uint16) ([]byte, error) {
	idBytes, err := a.id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal inner id: %w", err)
	}
	writer := bytes.NewBuffer(make([]byte, 0, int(unsafe.Sizeof(round))+len(idBytes)))
	if n, err := writer.Write(idBytes); err != nil || n != len(idBytes) {
		return nil, fmt.Errorf("unable to write inner id to coin seed: %w", err)
	} else if err := binary.Write(writer, binary.LittleEndian, round); err != nil {
		return nil, fmt.Errorf("unable to write round to coin seed: %w", err)
	}
	return writer.Bytes(), nil
}

func (a *abaNetworkedInstance) submitEcho(echo byte, sender uuid.UUID, r uint16) error {
	abaNetworkedLogger.Debug("submitting echo", "instance", a.id, "round", r, "echo", echo)
	if err := a.instance.submitEcho(echo, sender, r); err != nil {
		return fmt.Errorf("unable to submit echo: %w", err)
	}
	return nil
}

func (a *abaNetworkedInstance) submitVote(vote byte, sender uuid.UUID, r uint16) error {
	abaNetworkedLogger.Debug("submitting vote", "instance", a.id, "round", r, "vote", vote)
	if err := a.instance.submitVote(vote, sender, r); err != nil {
		return fmt.Errorf("unable to submit vote: %w", err)
	}
	return nil
}

func (a *abaNetworkedInstance) submitDecision(decision byte, sender uuid.UUID) error {
	abaNetworkedLogger.Debug("submitting decision", "instance", a.id, "decision", decision, "sender", sender)
	err := a.instance.submitDecision(decision, sender)
	if err != nil {
		return fmt.Errorf("unable to submit decision: %w", err)
	}
	return nil
}

func (a *abaNetworkedInstance) canOutputDecision() bool {
	a.deliveryLock.Lock()
	defer a.deliveryLock.Unlock()
	if !a.hasDelivered {
		a.hasDelivered = true
		return true
	}
	return false
}

func (a *abaNetworkedInstance) outputDecision(decision byte) {
	a.decisionChan <- decision
	if err := a.termidware.broadcastDecision(a.id, decision); err != nil {
		abaNetworkedLogger.Warn("unable to broadcast decision", "instance", a.id, "decision", decision, "error", err)
	}
}

func (a *abaNetworkedInstance) close() {
	if a.instance != nil {
		a.instance.close()
		abaNetworkedLogger.Debug("signaling close listener", "instance", a.id)
		a.listenerClose <- struct{}{}
	}
}
