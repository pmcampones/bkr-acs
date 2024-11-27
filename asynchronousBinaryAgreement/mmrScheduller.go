package asynchronousBinaryAgreement

import (
	"bkr-acs/utils"
	"crypto/sha256"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"math/rand"
	"testing"
	"time"
)

var mmrSchedulerLogger = utils.GetLogger("MMR Test Scheduler", slog.LevelDebug)

type wrappedMMR struct {
	m        *concurrentMMR
	decision chan byte
}

type mmrScheduler interface {
	addInstance(m *concurrentMMR) *wrappedMMR
	getChannels(n, f uint, sender uuid.UUID) *wrappedMMR
}

type mmrOrderedScheduler struct {
	t         *testing.T
	instances []*wrappedMMR
}

func newMMROrderedScheduler(t *testing.T) *mmrOrderedScheduler {
	return &mmrOrderedScheduler{
		t:         t,
		instances: make([]*wrappedMMR, 0),
	}
}

func (o *mmrOrderedScheduler) addInstance(m *concurrentMMR) *wrappedMMR {
	wmmr := &wrappedMMR{
		m:        m,
		decision: make(chan byte),
	}
	o.instances = append(o.instances, wmmr)
	return wmmr
}

func (o *mmrOrderedScheduler) getChannels(n, f uint, sender uuid.UUID) *wrappedMMR {
	m := newConcurrentMMR(n, f)
	wmmr := o.addInstance(m)
	go o.listenBVals(o.t, m.handler.deliverEcho, sender)
	go o.listenAux(o.t, m.handler.deliverVote, sender)
	go o.listenDecisions(o.t, wmmr, sender)
	go o.listenCoinRequests(o.t, m.handler.coinReq, m)
	return wmmr
}

func (o *mmrOrderedScheduler) listenBVals(t *testing.T, bValChan chan roundMsg, sender uuid.UUID) {
	func() {
		for {
			bVal := <-bValChan
			for _, wmmr := range o.instances {
				go func() {
					assert.NoError(t, wmmr.m.submitEcho(bVal.val, sender, bVal.r))
				}()
			}
		}
	}()
}

func (o *mmrOrderedScheduler) listenAux(t *testing.T, auxChan chan roundMsg, sender uuid.UUID) {
	func() {
		for {
			aux := <-auxChan
			for _, wmmr := range o.instances {
				go func() {
					assert.NoError(t, wmmr.m.submitVote(aux.val, sender, aux.r))
				}()
			}
		}
	}()
}

func (o *mmrOrderedScheduler) listenDecisions(t *testing.T, instance *wrappedMMR, sender uuid.UUID) {
	func() {
		decision := <-instance.m.handler.deliverDecision
		for _, wmmr := range o.instances {
			go func() {
				err := wmmr.m.submitDecision(decision, sender)
				assert.NoError(t, err)
			}()
		}
		instance.decision <- decision
		dec2 := <-instance.m.handler.deliverDecision
		t.Errorf("received a decision from the same inner twice: %d", dec2)
	}()
}

func (o *mmrOrderedScheduler) listenCoinRequests(t *testing.T, coinChan chan uint16, m *concurrentMMR) {
	func() {
		for {
			round := <-coinChan
			coinBool := hashToBool([]byte(fmt.Sprintf("%d", round)))
			coin := byte(0)
			if coinBool {
				coin = 1
			}
			go func() {
				assert.NoError(t, m.submitCoin(coin, round))
			}()
		}
	}()
}

type mmrUnorderedScheduler struct {
	t            *testing.T
	instances    []*wrappedMMR
	scheduleChan chan func() error
	ops          []func() error
	ticker       *time.Ticker
}

func newMMRUnorderedScheduler(t *testing.T) *mmrUnorderedScheduler {
	r := rand.New(rand.NewSource(0))
	ticker := time.NewTicker(1 * time.Millisecond)
	s := &mmrUnorderedScheduler{
		t:            t,
		instances:    make([]*wrappedMMR, 0),
		scheduleChan: make(chan func() error),
		ops:          make([]func() error, 0),
		ticker:       ticker,
	}
	go func() {
		for {
			select {
			case <-ticker.C:
				s.execOp(t)
			case op := <-s.scheduleChan:
				s.reorderOps(op, r)
			}
		}
	}()
	return s
}

func (u *mmrUnorderedScheduler) reorderOps(op func() error, r *rand.Rand) {
	u.ops = append(u.ops, op)
	mmrSchedulerLogger.Debug("reordering ops", "num ops", len(u.ops))
	r.Shuffle(len(u.ops), func(i, j int) { u.ops[i], u.ops[j] = u.ops[j], u.ops[i] })
}

func (u *mmrUnorderedScheduler) execOp(t *testing.T) {
	if len(u.ops) > 0 {
		op := u.ops[0]
		u.ops = u.ops[1:]
		go assert.NoError(t, op())
	}
}

func (u *mmrUnorderedScheduler) addInstance(m *concurrentMMR) *wrappedMMR {
	wmmr := &wrappedMMR{
		m:        m,
		decision: make(chan byte, 1),
	}
	u.instances = append(u.instances, wmmr)
	return wmmr
}

func (u *mmrUnorderedScheduler) getChannels(n, f uint, sender uuid.UUID) *wrappedMMR {
	m := newConcurrentMMR(n, f)
	wmmr := u.addInstance(m)
	go u.listenBVals(m.handler.deliverEcho, sender)
	go u.listenAux(m.handler.deliverVote, sender)
	go u.listenDecisions(u.t, wmmr, sender)
	go u.listenCoinRequests(m.handler.coinReq, m)
	return wmmr
}

func (u *mmrUnorderedScheduler) listenBVals(bValChan chan roundMsg, sender uuid.UUID) {
	func() {
		for {
			bVal := <-bValChan
			for _, wmmr := range u.instances {
				go func() {
					u.scheduleChan <- func() error {
						return wmmr.m.submitEcho(bVal.val, sender, bVal.r)
					}
				}()
			}
		}
	}()
}

func (u *mmrUnorderedScheduler) listenAux(auxChan chan roundMsg, sender uuid.UUID) {
	func() {
		for {
			aux := <-auxChan
			for _, wmmr := range u.instances {
				go func() {
					u.scheduleChan <- func() error {
						return wmmr.m.submitVote(aux.val, sender, aux.r)
					}
				}()
			}
		}
	}()
}

func (u *mmrUnorderedScheduler) listenDecisions(t *testing.T, instance *wrappedMMR, sender uuid.UUID) {
	func() {
		decision := <-instance.m.handler.deliverDecision
		mmrSchedulerLogger.Info("received inner decision", "decision", decision, "sender", sender)
		for _, wmmr := range u.instances {
			go func() {
				u.scheduleChan <- func() error {
					mmrSchedulerLogger.Debug("submitting decision", "decision", decision, "sender", sender)
					if err := wmmr.m.submitDecision(decision, sender); err != nil {
						return err
					}
					return nil
				}
			}()
			instance.decision <- decision
		}
		dec2 := <-instance.m.handler.deliverDecision
		t.Errorf("received a decision from the same inner twice: %d", dec2)
	}()
}

func (u *mmrUnorderedScheduler) listenCoinRequests(coinChan chan uint16, m *concurrentMMR) {
	func() {
		for {
			round := <-coinChan
			coinBool := hashToBool([]byte(fmt.Sprintf("%d", round)))
			coin := byte(0)
			if coinBool {
				coin = 1
			}
			go func() {
				u.scheduleChan <- func() error {
					return m.submitCoin(coin, round)
				}
			}()
		}
	}()
}

func hashToBool(seed []byte) bool {
	hash := sha256.Sum256(seed)
	return hash[0]%2 == 0
}
