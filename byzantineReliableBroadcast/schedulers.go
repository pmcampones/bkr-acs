package byzantineReliableBroadcast

import (
	"bkr-acs/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"math/rand"
	"testing"
	"time"
)

var schedulerLogger = utils.GetLogger("BRB Test Schedulers", slog.LevelDebug)

type scheduler interface {
	getChannels(t *testing.T, sender uuid.UUID) (chan []byte, chan []byte)
	sendAll(t *testing.T, msg []byte)
	addInstance(instance *brbInstance)
	getInstances() []*brbInstance
}

type orderedScheduler struct {
	instances []*brbInstance
}

func newOrderedScheduler() *orderedScheduler {
	return &orderedScheduler{instances: make([]*brbInstance, 0)}
}

func (o *orderedScheduler) getChannels(t *testing.T, sender uuid.UUID) (chan []byte, chan []byte) {
	echoChan := make(chan []byte)
	readyChan := make(chan []byte)
	go func() {
		msg := <-echoChan
		for i, inst := range o.instances {
			schedulerLogger.Debug("executing echo", "sender", sender, "instance", i)
			assert.NoError(t, inst.echo(msg, sender))
		}
	}()
	go func() {
		msg := <-readyChan
		for i, inst := range o.instances {
			schedulerLogger.Debug("executing ready", "sender", sender, "instance", i)
			assert.NoError(t, inst.ready(msg, sender))
		}
	}()
	return echoChan, readyChan
}

func (o *orderedScheduler) sendAll(t *testing.T, msg []byte) {
	schedulerLogger.Info("sending all", "msg", string(msg))
	sender := uuid.New()
	for i, inst := range o.instances {
		schedulerLogger.Debug("executing send", "instance", i)
		assert.NoError(t, inst.send(msg, sender))
	}
}

func (o *orderedScheduler) addInstance(instance *brbInstance) {
	o.instances = append(o.instances, instance)
}

func (o *orderedScheduler) getInstances() []*brbInstance {
	return o.instances
}

type unorderedScheduler struct {
	instances    []*brbInstance
	scheduleChan chan func()
	ops          []func()
	ticker       *time.Ticker
}

func newUnorderedScheduler() *unorderedScheduler {
	r := rand.New(rand.NewSource(0))
	ticker := time.NewTicker(3 * time.Millisecond)
	scheduler := &unorderedScheduler{
		instances:    make([]*brbInstance, 0),
		scheduleChan: make(chan func()),
		ops:          make([]func(), 0),
		ticker:       ticker,
	}
	go func() {
		for {
			select {
			case <-ticker.C:
				scheduler.execOp()
			case op := <-scheduler.scheduleChan:
				scheduler.reorderOps(op, r)
			}
		}
	}()
	return scheduler
}

func (u *unorderedScheduler) reorderOps(op func(), r *rand.Rand) {
	u.ops = append(u.ops, op)
	schedulerLogger.Debug("reordering ops", "num ops", len(u.ops))
	r.Shuffle(len(u.ops), func(i, j int) { u.ops[i], u.ops[j] = u.ops[j], u.ops[i] })
}

func (u *unorderedScheduler) execOp() {
	if len(u.ops) > 0 {
		op := u.ops[0]
		u.ops = u.ops[1:]
		go op()
	}
}

func (u *unorderedScheduler) getChannels(t *testing.T, sender uuid.UUID) (chan []byte, chan []byte) {
	echoChan := make(chan []byte)
	readyChan := make(chan []byte)
	go func() {
		msg := <-echoChan
		for i, inst := range u.instances {
			u.scheduleChan <- func() {
				schedulerLogger.Debug("executing echo", "sender", sender, "instance", i)
				assert.NoError(t, inst.echo(msg, sender))
			}
		}
	}()
	go func() {
		msg := <-readyChan
		for i, inst := range u.instances {
			u.scheduleChan <- func() {
				schedulerLogger.Debug("executing ready", "sender", sender, "instance", i)
				assert.NoError(t, inst.ready(msg, sender))
			}
		}
	}()
	return echoChan, readyChan
}

func (u *unorderedScheduler) sendAll(t *testing.T, msg []byte) {
	schedulerLogger.Info("sending all", "msg", string(msg))
	sender := uuid.New()
	for i, inst := range u.instances {
		u.scheduleChan <- func() {
			schedulerLogger.Debug("executing send", "instance", i)
			assert.NoError(t, inst.send(msg, sender))
		}
	}
}

func (u *unorderedScheduler) addInstance(instance *brbInstance) {
	u.instances = append(u.instances, instance)
}

func (u *unorderedScheduler) getInstances() []*brbInstance {
	return u.instances
}

func (u *unorderedScheduler) stop() {
	schedulerLogger.Info("stopping scheduler")
	u.ticker.Stop()
}

func instantiateCorrect(t *testing.T, outputChans []chan BRBMsg, scheduler scheduler, n, f uint) {
	for _, o := range outputChans {
		echoChan, readyChan := scheduler.getChannels(t, uuid.New())
		instance := newBrbInstance(n, f, echoChan, readyChan, o)
		scheduler.addInstance(instance)
	}
}
