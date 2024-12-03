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
	mmrRound
	closeChan chan struct{}
}

func (r *cancelableRound) close() {
	r.closeChan <- struct{}{}
}

type mmr struct {
	n                      uint
	f                      uint
	deliverEcho            chan roundMsg
	deliverVote            chan roundMsg
	deliverBind            chan roundMsg
	deliverExternallyValid chan roundMsg
	reachedDecision        chan byte
	deliverDecision        chan byte
	hasDecided             bool
	coinReq                chan uint16
	rounds                 map[uint16]*cancelableRound
	termGadget             *mmrTermination
}

func newMMR(n, f uint) mmr {
	m := mmr{
		n:                      n,
		f:                      f,
		deliverEcho:            make(chan roundMsg, 2*(averageNumRounds+1)),
		deliverVote:            make(chan roundMsg, averageNumRounds+1),
		deliverBind:            make(chan roundMsg, averageNumRounds+1),
		deliverExternallyValid: make(chan roundMsg, 2*(averageNumRounds+1)),
		reachedDecision:        make(chan byte, 1),
		deliverDecision:        make(chan byte, 1),
		hasDecided:             false,
		coinReq:                make(chan uint16, averageNumRounds+1),
		rounds:                 make(map[uint16]*cancelableRound),
		termGadget:             newMmrTermination(n, f),
	}
	m.initFirstRound()
	go m.reachDecision()
	return m
}

func (m *mmr) propose(est, prevCoin byte, r uint16) error {
	abaLogger.Debug("proposing estimate", "est", est, "round", r)
	round := m.getRound(r)
	if err := round.propose(est, prevCoin); err != nil {
		return fmt.Errorf("unable to propose to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitEcho(echo byte, sender uuid.UUID, r uint16) error {
	abaLogger.Debug("submitting echo", "echo", echo, "sender", sender, "round", r)
	round := m.getRound(r)
	if err := round.submitEcho(echo, sender); err != nil {
		return fmt.Errorf("unable to submit echo to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitVote(vote byte, sender uuid.UUID, r uint16) error {
	abaLogger.Debug("submitting vote", "vote", vote, "sender", sender, "round", r)
	round := m.getRound(r)
	if err := round.submitVote(vote, sender); err != nil {
		return fmt.Errorf("unable to submit vote to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitBind(bind byte, sender uuid.UUID, r uint16) error {
	abaLogger.Debug("submitting bind", "bind", bind, "sender", sender, "round", r)
	round := m.getRound(r)
	if err := round.submitBind(bind, sender); err != nil {
		return fmt.Errorf("unable to submit bind to round %d: %v", r, err)
	}
	return nil
}

func (m *mmr) submitCoin(coin byte, r uint16) error {
	abaLogger.Debug("submitting coin", "coin", coin, "mmrRound", r)
	round := m.getRound(r)
	if res := round.submitCoin(coin); res.err != nil {
		return fmt.Errorf("unable to submit coin to round %d: %v", r, res.err)
	} else if res.decided && !m.hasDecided {
		m.hasDecided = true
		m.reachedDecision <- res.estimate
	} else if err := m.propose(res.estimate, coin, r+1); err != nil {
		return fmt.Errorf("unable to propose to round %d: %v", r+1, err)
	}
	return nil
}

func (m *mmr) submitExternallyValid(val byte, r uint16) {
	abaLogger.Debug("submitting externally valid value", "val", val, "round", r)
	round := m.getRound(r + 1)
	round.submitExternallyValid(val)
}

func (m *mmr) getRound(rNum uint16) *cancelableRound {
	round := m.rounds[rNum]
	if round == nil {
		r := m.newRound(rNum)
		round = r
		m.rounds[rNum] = r
	}
	return round
}

func (m *mmr) newRound(r uint16) *cancelableRound {
	round := newRound(m.n, m.f)
	closeChan := make(chan struct{}, 1)
	go m.listenRequests(&round, closeChan, r)
	return &cancelableRound{
		mmrRound:  round,
		closeChan: closeChan,
	}
}

func (m *mmr) initFirstRound() {
	round := newFirstRound(m.n, m.f)
	closeChan := make(chan struct{}, 1)
	go m.listenRequests(&round, closeChan, firstRound)
	m.rounds[firstRound] = &cancelableRound{
		mmrRound:  round,
		closeChan: closeChan,
	}
}

func (m *mmr) listenRequests(round *mmrRound, close chan struct{}, rnum uint16) {
	for {
		select {
		case echo := <-round.getBcastEchoChan():
			abaLogger.Debug("broadcasting echo", "echo", echo, "round", rnum)
			go func() {
				m.deliverEcho <- roundMsg{val: echo, r: rnum}
			}()
		case vote := <-round.getBcastVoteChan():
			abaLogger.Debug("broadcasting vote", "vote", vote, "round", rnum)
			go func() {
				m.deliverVote <- roundMsg{val: vote, r: rnum}
			}()
		case bind := <-round.getBcastBindChan():
			abaLogger.Debug("broadcasting bind", "bind", bind, "round", rnum)
			go func() {
				m.deliverBind <- roundMsg{val: bind, r: rnum}
			}()
		case <-round.coinReqChan:
			abaLogger.Debug("coin request", "round", rnum)
			go func() {
				m.coinReq <- rnum
			}()
		case val := <-round.getOutputExternalValidChan():
			abaLogger.Debug("external valid value", "val", val, "round", rnum)
			go func() {
				m.deliverExternallyValid <- roundMsg{val: val, r: rnum}
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
