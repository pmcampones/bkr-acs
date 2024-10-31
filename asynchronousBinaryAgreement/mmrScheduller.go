package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"pace/utils"
	"testing"
	"time"
)

type wrappedMMR struct {
	m        *mmr
	decision chan byte
}

type mmrScheduler struct {
	maxIntervalMillis int
	instances         []*wrappedMMR
}

func newMMRScheduler(maxIntervalMillis int) *mmrScheduler {
	return &mmrScheduler{
		maxIntervalMillis: maxIntervalMillis,
		instances:         make([]*wrappedMMR, 0),
	}
}

func (os *mmrScheduler) addInstance(m *mmr) *wrappedMMR {
	wmmr := &wrappedMMR{
		m:        m,
		decision: make(chan byte),
	}
	os.instances = append(os.instances, wmmr)
	return wmmr
}

func (os *mmrScheduler) getChannels(t *testing.T, n, f uint, sender uuid.UUID) *wrappedMMR {
	bValChan := make(chan roundMsg)
	auxChan := make(chan roundMsg)
	decisionChan := make(chan byte)
	coinChan := make(chan uint16)
	m := newMMR(n, f, bValChan, auxChan, decisionChan, coinChan)
	wmmr := os.addInstance(m)
	go os.listenBVals(t, bValChan, sender)
	go os.listenAux(t, auxChan, sender)
	go os.listenDecisions(t, decisionChan, sender)
	go os.listenCoinRequests(t, coinChan, m)
	return wmmr
}

func (os *mmrScheduler) listenBVals(t *testing.T, bValChan chan roundMsg, sender uuid.UUID) {
	func() {
		for {
			bVal := <-bValChan
			for _, wmmr := range os.instances {
				go func() {
					os.sleep()
					assert.NoError(t, wmmr.m.submitBVal(bVal.val, sender, bVal.r))
				}()
			}
		}
	}()
}

func (os *mmrScheduler) listenAux(t *testing.T, auxChan chan roundMsg, sender uuid.UUID) {
	func() {
		for {
			aux := <-auxChan
			for _, wmmr := range os.instances {
				go func() {
					os.sleep()
					assert.NoError(t, wmmr.m.submitAux(aux.val, sender, aux.r))
				}()
			}
		}
	}()
}

func (os *mmrScheduler) listenDecisions(t *testing.T, decisionChan chan byte, sender uuid.UUID) {
	func() {
		decision := <-decisionChan
		for _, wmmr := range os.instances {
			go func() {
				os.sleep()
				dec, err := wmmr.m.submitDecision(decision, sender)
				assert.NoError(t, err)
				if dec != bot {
					wmmr.decision <- dec
				}
			}()
		}
		dec2 := <-decisionChan
		t.Errorf("received a decision from the same instance twice: %d", dec2)
	}()
}

func (os *mmrScheduler) listenCoinRequests(t *testing.T, coinChan chan uint16, m *mmr) {
	func() {
		for {
			round := <-coinChan
			coinBool := utils.HashToBool([]byte(fmt.Sprintf("%d", round)))
			coin := byte(0)
			if coinBool {
				coin = 1
			}
			go func() {
				os.sleep()
				assert.NoError(t, m.submitCoin(coin, round))
			}()
		}
	}()
}

func (os *mmrScheduler) sleep() {
	if os.maxIntervalMillis > 0 {
		time.Sleep(time.Duration(os.maxIntervalMillis) * time.Millisecond)
	}
}
