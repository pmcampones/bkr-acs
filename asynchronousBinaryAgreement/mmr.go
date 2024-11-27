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
	deliverEcho     chan roundMsg
	deliverVote     chan roundMsg
	reachedDecision chan byte
	deliverDecision chan byte
	hasDecided      bool
	coinReq         chan uint16
	rounds          map[uint16]*cancelableRound
	termGadget      *mmrTermination
}

func newMMR(n, f uint) *mmr {
	m := &mmr{
		n:               n,
		f:               f,
		deliverEcho:     make(chan roundMsg, 2*(averageNumRounds+1)),
		deliverVote:     make(chan roundMsg, averageNumRounds+1),
		reachedDecision: make(chan byte, 1),
		deliverDecision: make(chan byte, 1),
		hasDecided:      false,
		coinReq:         make(chan uint16, averageNumRounds+1),
		rounds:          make(map[uint16]*cancelableRound),
		termGadget:      newMmrTermination(n, f),
	}
	go m.reachDecision()
	return m
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

func (m *mmr) submitEcho(echo byte, sender uuid.UUID, r uint16) error {
	abaLogger.Debug("submitting echo", "echo", echo, "sender", sender, "round", r)
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.submitEcho(echo, sender); err != nil {
		return fmt.Errorf("unable to submit echo to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitVote(vote byte, sender uuid.UUID, r uint16) error {
	abaLogger.Debug("submitting vote", "vote", vote, "sender", sender, "round", r)
	if round, err := m.getRound(r); err != nil {
		return fmt.Errorf("unable to get round %d: %v", r, err)
	} else if err := round.round.submitVote(vote, sender); err != nil {
		return fmt.Errorf("unable to submit vote to round %d: %v", r, err)
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
		m.reachedDecision <- res.estimate
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
		case echo := <-round.getEchoChan():
			abaLogger.Debug("broadcasting echo", "echo", echo, "round", rnum)
			go func() {
				m.deliverEcho <- roundMsg{val: echo, r: rnum}
			}()
		case vote := <-round.getVoteChan():
			abaLogger.Debug("broadcasting vote", "vote", vote, "round", rnum)
			go func() {
				m.deliverVote <- roundMsg{val: vote, r: rnum}
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

func (m *mmr) submitDecision(decision byte, sender uuid.UUID) error {
	if err := m.termGadget.submitDecision(decision, sender); err != nil {
		return fmt.Errorf("unable to submit decision: %v", err)
	}
	return nil
}

func (m *mmr) reachDecision() {
	var decision byte
	select {
	case roundDec := <-m.reachedDecision:
		decision = roundDec
	case termDec := <-m.termGadget.deliverDecision:
		decision = termDec
	}
	m.deliverDecision <- decision
}

func (m *mmr) close() {
	for _, round := range m.rounds {
		round.close()
	}
}
