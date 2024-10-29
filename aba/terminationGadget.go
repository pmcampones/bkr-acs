package aba

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/brb"
	"pace/utils"
)

var termLogger = utils.GetLogger(slog.LevelDebug)

type terminationGadget struct {
	roundPrefix []byte
	brb         *brb.BRBChannel
	received    map[uuid.UUID]bool
	results     []uint
	f           uint
	output      chan byte
}

func newTerminationGadget(roundPrefix []byte, brb *brb.BRBChannel, f uint) *terminationGadget {
	tg := &terminationGadget{
		roundPrefix: roundPrefix,
		brb:         brb,
		received:    make(map[uuid.UUID]bool),
		results:     []uint{0, 0},
		f:           f,
		output:      make(chan byte),
	}
	go tg.listenDecisions()
	return tg
}

func (tg *terminationGadget) listenDecisions() {
	for brbMsg := range tg.brb.BrbDeliver {
		msg := brbMsg.Content
		if tg.pertainsThisRound(msg) {
			err := tg.processMsg(msg[len(tg.roundPrefix):], brbMsg.Sender)
			if err != nil {
				termLogger.Warn("unable to process message", "error", err)
			}
		}
	}
}

func (tg *terminationGadget) processMsg(msg []byte, sender uuid.UUID) error {
	if tg.received[sender] {
		return fmt.Errorf("duplicate message from %s", sender)
	} else if len(msg) != 1 {
		return fmt.Errorf("invalid message length %d, expected single byte", len(msg))
	} else if decision := msg[0]; decision > 1 {
		return fmt.Errorf("invalid decision %d", decision)
	} else {
		tg.received[sender] = true
		tg.results[decision]++
		if tg.results[decision] > tg.f {
			tg.output <- decision
		}
	}
	return nil
}

func (tg *terminationGadget) pertainsThisRound(msg []byte) bool {
	return len(msg) >= len(tg.roundPrefix) && bytes.Equal(msg[:len(tg.roundPrefix)], tg.roundPrefix)
}

func (tg *terminationGadget) decide(decision byte) error {
	if err := tg.brb.BRBroadcast(append(tg.roundPrefix, decision)); err != nil {
		return fmt.Errorf("unable to broadcast decision: %w", err)
	}
	return nil
}
