package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var abaLogger = utils.GetLogger("MMR Instance", slog.LevelWarn)

const firstRound = 0
const averageNumRounds = 2

type roundMsg struct {
	val byte
	r   uint16
}

type cancelableRound struct {
	round     *mmrRound
	closeChan chan struct{}
}

func (r *cancelableRound) close() {
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
	rounds          map[uint16]*cancelableRound
	termGadget      *mmrTermination
}

func newMMR(n, f uint) *mmr {
	return &mmr{
		n:               n,
		f:               f,
		deliverBVal:     make(chan roundMsg, 2*(averageNumRounds+1)),
		deliverAux:      make(chan roundMsg, averageNumRounds+1),
		deliverDecision: make(chan byte, 1),
		hasDecided:      false,
		coinReq:         make(chan uint16, averageNumRounds+1),
		rounds:          make(map[uint16]*cancelableRound),
		termGadget:      newMmrTermination(n, f),
	}
}

func (m *mmr) propose(est byte, r uint16) error {
	abaLogger.Debug("proposing estimate", "est", est, "round", r)
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.propose(est); err != nil {
		return fmt.Errorf("unable to propose to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitBVal(bVal byte, sender uuid.UUID, r uint16) error {
	abaLogger.Debug("submitting bVal", "bVal", bVal, "sender", sender, "round", r)
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.submitBVal(bVal, sender); err != nil {
		return fmt.Errorf("unable to submit bVal to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitAux(aux byte, sender uuid.UUID, r uint16) error {
	abaLogger.Debug("submitting aux", "aux", aux, "sender", sender, "round", r)
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.submitAux(aux, sender); err != nil {
		return fmt.Errorf("unable to submit aux to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitCoin(coin byte, r uint16) error {
	abaLogger.Debug("submitting coin", "coin", coin, "mmrRound", r)
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if res := round.round.submitCoin(coin); res.err != nil {
		return fmt.Errorf("unable to submit coin to round %d: %v", r, res.err)
	} else if res.decided && !m.hasDecided {
		m.hasDecided = true
		m.deliverDecision <- res.estimate
	} else if err := m.propose(res.estimate, r+1); err != nil {
		return fmt.Errorf("unable to propose to round %d: %v", r+1, err)
	}
	return nil
}

func (m *mmr) getRound(rNum uint16) (*cancelableRound, error) {
	round := m.rounds[rNum]
	if round == nil {
		if r, err := m.newRound(rNum); err != nil {
			return nil, fmt.Errorf("unable to create new round: %v", err)
		} else {
			round = r
			m.rounds[rNum] = r
		}
	}
	return round, nil
}

func (m *mmr) newRound(r uint16) (*cancelableRound, error) {
	round := newMMRRound(m.n, m.f)
	closeChan := make(chan struct{}, 1)
	go m.listenRequests(round, closeChan, r)
	return &cancelableRound{
		round:     round,
		closeChan: closeChan,
	}, nil
}

func (m *mmr) listenRequests(round *mmrRound, close chan struct{}, rnum uint16) {
	for {
		select {
		case bVal := <-round.bValChan:
			abaLogger.Debug("broadcasting bVal", "bVal", bVal, "round", rnum)
			go func() {
				m.deliverBVal <- roundMsg{val: bVal, r: rnum}
			}()
		case aux := <-round.auxChan:
			abaLogger.Debug("broadcasting aux", "aux", aux, "round", rnum)
			go func() {
				m.deliverAux <- roundMsg{val: aux, r: rnum}
			}()
		case <-round.coinReqChan:
			abaLogger.Debug("coin request", "round", rnum)
			go func() {
				m.coinReq <- rnum
			}()
		case <-close:
			abaLogger.Info("closing concurrentMMR round", "round", rnum)
			return
		}
	}
}

func (m *mmr) submitDecision(decision byte, sender uuid.UUID) (byte, error) {
	res, err := m.termGadget.submitDecision(decision, sender)
	if err != nil {
		return bot, fmt.Errorf("unable to submit decision: %v", err)
	} else if res != bot && !m.hasDecided {
		m.hasDecided = true
		m.deliverDecision <- res
	}
	return res, nil
}

func (m *mmr) close() {
	for _, round := range m.rounds {
		round.close()
	}
}
