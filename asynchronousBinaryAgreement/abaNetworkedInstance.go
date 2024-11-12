package asynchronousBinaryAgreement

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	ct "pace/coinTosser"
	"unsafe"
)

type abaNetworkedInstance struct {
	id            uuid.UUID
	instance      *concurrentMMR
	output        chan byte
	abamidware    *abaMiddleware
	termidware    *terminationMiddleware
	ctChan        *ct.CTChannel
	listenerClose chan struct{}
}

func newAbaNetworkedInstance(id uuid.UUID, n, f uint, abamidware *abaMiddleware, termidware *terminationMiddleware, ctChan *ct.CTChannel) *abaNetworkedInstance {
	a := &abaNetworkedInstance{
		id:            id,
		instance:      newConcurrentMMR(n, f),
		output:        make(chan byte),
		abamidware:    abamidware,
		termidware:    termidware,
		ctChan:        ctChan,
		listenerClose: make(chan struct{}),
	}
	go a.listener()
	return a
}

func (a *abaNetworkedInstance) propose(est byte) error {
	abaChannelLogger.Debug("proposing initial estimate", "instanceId", a.id, "est", est)
	if err := a.instance.propose(est); err != nil {
		return fmt.Errorf("unable to propose initial estimate: %w", err)
	}
	return nil
}

func (a *abaNetworkedInstance) listener() {
	abaChannelLogger.Debug("starting listener aba networked inner")
	for {
		select {
		case bVal := <-a.instance.getBValChan():
			if err := a.abamidware.broadcastBVal(a.id, bVal.r, bVal.val); err != nil {
				abaChannelLogger.Warn("unable to broadcast bVal", "instanceId", a.id, "round", bVal.r, "error", err)
			}
		case aux := <-a.instance.getAuxChan():
			if err := a.abamidware.broadcastAux(a.id, aux.r, aux.val); err != nil {
				abaChannelLogger.Warn("unable to broadcast aux", "instanceId", a.id, "round", aux.r, "error", err)
			}
		case decision := <-a.instance.getDecisionChan():
			if err := a.termidware.broadcastDecision(a.id, decision); err != nil {
				abaChannelLogger.Warn("unable to broadcast decision", "instanceId", a.id, "decision", decision, "error", err)
			}
		case coinReq := <-a.instance.getCoinReqChan():
			coin, err := a.getCoin(coinReq)
			if err != nil {
				abaChannelLogger.Warn("unable to get coin", "instanceId", a.id, "round", coinReq, "error", err)
			} else if err := a.instance.submitCoin(coin, coinReq); err != nil {
				abaChannelLogger.Warn("unable to submit coin", "instanceId", a.id, "round", coinReq, "error", err)
			}
		case <-a.listenerClose:
			abaChannelLogger.Debug("closing listener asynchronousBinaryAgreement networked inner")
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
	abaChannelLogger.Debug("requesting coin", "instanceId", a.id, "round", round)
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

func (a *abaNetworkedInstance) submitBVal(bVal byte, sender uuid.UUID, r uint16) error {
	abaChannelLogger.Debug("submitting bVal", "instanceId", a.id, "round", r, "bval", bVal)
	if err := a.instance.submitBVal(bVal, sender, r); err != nil {
		return fmt.Errorf("unable to submit bVal: %w", err)
	}
	return nil
}

func (a *abaNetworkedInstance) submitAux(aux byte, sender uuid.UUID, r uint16) error {
	abaChannelLogger.Debug("submitting aux", "instanceId", a.id, "round", r, "aux", aux)
	if err := a.instance.submitAux(aux, sender, r); err != nil {
		return fmt.Errorf("unable to submit aux: %w", err)
	}
	return nil
}

func (a *abaNetworkedInstance) submitDecision(decision byte, sender uuid.UUID) error {
	abaChannelLogger.Debug("submitting decision", "instanceId", a.id, "decision", decision, "sender", sender)
	finalDec, err := a.instance.submitDecision(decision, sender)
	if err != nil {
		return fmt.Errorf("unable to submit decision: %w", err)
	}
	if finalDec != bot {
		abaChannelLogger.Debug("final decision", "instanceId", a.id, "decision", finalDec)
		a.output <- finalDec
	}
	return nil
}

func (a *abaNetworkedInstance) close() {
	if a.instance != nil {
		a.instance.close()
		abaChannelLogger.Debug("signaling close asynchronousBinaryAgreement networked inner")
		a.listenerClose <- struct{}{}
	}
}
