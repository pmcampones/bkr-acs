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

func newTerminationGadget(roundPrefix []byte, brb *brb.BRBChannel, f uint, output chan byte) *terminationGadget {
	tg := &terminationGadget{
		roundPrefix: roundPrefix,
		brb:         brb,
		received:    make(map[uuid.UUID]bool),
		results:     []uint{0, 0},
		f:           f,
		output:      output,
	}
	go tg.listenDecisions()
	return tg
}

func (tg *terminationGadget) listenDecisions() {
	for brbMsg := range tg.brb.BrbDeliver {
		msg := brbMsg.Content
		if tg.pertainsThisRound(msg) {
			msg = msg[len(tg.roundPrefix):]
			if tg.received[brbMsg.Sender] {
				termLogger.Warn("duplicate decision", "sender", brbMsg.Sender)
				continue
			} else if decision := msg[0]; decision > 1 {
				termLogger.Warn("invalid decision", "decision", decision)
				continue
			} else {
				tg.received[brbMsg.Sender] = true
				tg.results[decision]++
				if tg.results[decision] > tg.f {
					tg.output <- decision
					return
				}
			}
		}
	}
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
