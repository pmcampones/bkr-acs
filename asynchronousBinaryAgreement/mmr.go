package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var abaLogger = utils.GetLogger(slog.LevelDebug)

const firstRound = 0

type roundMsg struct {
	val byte
	r   uint16
}

type closeableRound struct {
	round     *mmrRound
	closeChan chan struct{}
}

func (r *closeableRound) close() {
	r.closeChan <- struct{}{}
}

type mmr struct {
	n               uint
	f               uint
	deliverBVal     chan roundMsg
	deliverAux      chan roundMsg
	deliverDecision chan byte
	hasDecided      bool
	coinReq         chan uint16
	rounds          map[uint16]*closeableRound
	termGadget      *mmrTermination
}

func newMMR(n, f uint, deliverBVal, deliverAux chan roundMsg, deliverDecision chan byte, coinReq chan uint16) *mmr {
	return &mmr{
		n:               n,
		f:               f,
		deliverBVal:     deliverBVal,
		deliverAux:      deliverAux,
		deliverDecision: deliverDecision,
		coinReq:         coinReq,
		rounds:          make(map[uint16]*closeableRound),
		termGadget:      newMmrTermination(f),
	}
}

func (m *mmr) propose(est byte) error {
	return m.proposeRound(est, firstRound)
}

func (m *mmr) proposeRound(est byte, r uint16) error {
	abaLogger.Debug("proposing estimate", "est", est, "mmrRound", r)
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.propose(est); err != nil {
		return fmt.Errorf("unable to propose to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitBVal(bVal byte, sender uuid.UUID, r uint16) error {
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.submitBVal(bVal, sender); err != nil {
		return fmt.Errorf("unable to submit bVal to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitAux(aux byte, sender uuid.UUID, r uint16) error {
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.submitAux(aux, sender); err != nil {
		return fmt.Errorf("unable to submit aux to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitCoin(coin byte, r uint16) error {
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if res := round.round.submitCoin(coin); res.err != nil {
		return fmt.Errorf("unable to submit coin to round %d: %v", r, res.err)
	} else if res.decided && !m.hasDecided {
		m.hasDecided = true
		m.deliverDecision <- res.estimate
	} else if err := m.proposeRound(res.estimate, r+1); err != nil {
		return fmt.Errorf("unable to proposeRound to round %d: %v", r+1, err)
	}
	return nil
}

func (m *mmr) submitDecision(decision byte, sender uuid.UUID) (byte, error) {
	output, err := m.termGadget.submitDecision(decision, sender)
	if err != nil {
		return bot, fmt.Errorf("unable to submit decision: %v", err)
	}
	return output, nil
}

func (m *mmr) getRound(rNum uint16) (*closeableRound, error) {
	round := m.rounds[rNum]
	if round == nil {
		round = m.newRound(rNum)
		m.rounds[rNum] = round
	}
	return round, nil
}

func (m *mmr) newRound(r uint16) *closeableRound {
	round := newMMRRound(m.n, m.f)
	closeChan := make(chan struct{}, 1)
	go m.listenRequests(round, closeChan, r)
	return &closeableRound{
		round:     round,
		closeChan: closeChan,
	}
}

func (m *mmr) listenRequests(round *mmrRound, close chan struct{}, rnum uint16) {
	for {
		select {
		case bVal := <-round.bValChan:
			abaLogger.Debug("received bVal", "bVal", bVal, "mmrRound", rnum)
			go func() {
				m.deliverBVal <- roundMsg{val: bVal, r: rnum}
			}()
		case aux := <-round.auxChan:
			abaLogger.Debug("received aux", "aux", aux, "mmrRound", rnum)
			go func() {
				m.deliverAux <- roundMsg{val: aux, r: rnum}
			}()
		case <-round.coinReqChan:
			abaLogger.Debug("received coin request", "mmrRound", rnum)
			go func() {
				m.coinReq <- rnum
			}()
		case <-close:
			abaLogger.Info("closing mmr round", "mmrRound", rnum)
			return
		}
	}
}

func (m *mmr) close() {
	m.termGadget.close()
	for _, round := range m.rounds {
		round.close()
	}
}
