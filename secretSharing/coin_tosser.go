package secretSharing

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/cloudflare/circl/group"
	. "github.com/google/uuid"
	"log/slog"
)

type coinObserver interface {
	observeCoin(id UUID, toss bool)
}

type coinToss struct {
	id            UUID
	threshold     uint
	base          group.Element
	deal          *Deal
	hiddenShares  []PointShare
	observers     []coinObserver
	peersReceived map[ecdsa.PublicKey]bool
	commands      chan<- func() error
	closeChan     chan struct{}
}

func newCoinToss(id UUID, threshold uint, base group.Element, deal *Deal) *coinToss {
	commands := make(chan func() error)
	ct := &coinToss{
		id:            id,
		threshold:     threshold,
		base:          base,
		deal:          deal,
		hiddenShares:  make([]PointShare, 0),
		observers:     make([]coinObserver, 0),
		peersReceived: make(map[ecdsa.PublicKey]bool),
		commands:      commands,
		closeChan:     make(chan struct{}),
	}
	go ct.invoker(commands, ct.closeChan)
	return ct
}

func (ct *coinToss) AttachObserver(observer coinObserver) {
	ct.observers = append(ct.observers, observer)
}

func (ct *coinToss) tossCoin() PointShare {
	return ShareToPoint(*ct.deal.share, ct.base)
}

func (ct *coinToss) getShare(shareBytes []byte, sender ecdsa.PublicKey) error {
	share, err := unmarshalPointShare(shareBytes)
	//todo: check if the share is valid
	if err != nil {
		return fmt.Errorf("unable to unmarshal share: %v", err)
	}
	ct.commands <- func() error {
		if ct.peersReceived[sender] {
			return fmt.Errorf("peer %v already sent share", sender)
		}
		ct.peersReceived[sender] = true
		return ct.processShare(share)
	}
	return nil
}

func (ct *coinToss) processShare(share PointShare) error {
	ct.hiddenShares = append(ct.hiddenShares, share)
	if len(ct.hiddenShares) == int(ct.threshold)+1 {
		secretPoint := RecoverSecretFromPoints(ct.hiddenShares)
		coinToss, err := HashPointToBool(secretPoint)
		if err != nil {
			return fmt.Errorf("unable to hash point to bool: %v", err)
		}
		for _, observer := range ct.observers {
			go observer.observeCoin(ct.id, coinToss)
		}
	}
	return nil
}

func (ct *coinToss) invoker(commands <-chan func() error, closeChan <-chan struct{}) {
	for {
		select {
		case command := <-commands:
			err := command()
			if err != nil {
				slog.Error("unable to compute command", "id", ct.id, "error", err)
			}
		case <-closeChan:
			slog.Debug("closing brb executor", "id", ct.id)
			return
		}
	}
}

func (ct *coinToss) close() {
	slog.Debug("sending signal to close bcb instance", "Id", ct.id)
	ct.closeChan <- struct{}{}
}
