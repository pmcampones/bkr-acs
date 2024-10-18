package brb

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

type scheduler interface {
	getChannels(t *testing.T, sender uuid.UUID) (chan []byte, chan []byte)
	sendAll(msg []byte)
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
		for _, inst := range o.instances {
			err := inst.echo(msg, sender)
			assert.NoError(t, err)
		}
	}()
	go func() {
		msg := <-readyChan
		for _, inst := range o.instances {
			err := inst.ready(msg, sender)
			assert.NoError(t, err)
		}
	}()
	return echoChan, readyChan
}

func (o *orderedScheduler) sendAll(msg []byte) {
	for _, i := range o.instances {
		i.send(msg)
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
	fmt.Println("reordering ops")
	u.ops = append(u.ops, op)
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
		for _, inst := range u.instances {
			u.scheduleChan <- func() {
				err := inst.echo(msg, sender)
				assert.NoError(t, err)
			}
		}
	}()
	go func() {
		msg := <-readyChan
		for _, inst := range u.instances {
			u.scheduleChan <- func() {
				err := inst.ready(msg, sender)
				assert.NoError(t, err)
			}
		}
	}()
	return echoChan, readyChan
}

func (u *unorderedScheduler) sendAll(msg []byte) {
	for _, i := range u.instances {
		u.scheduleChan <- func() { i.send(msg) }
	}
}

func (u *unorderedScheduler) addInstance(instance *brbInstance) {
	u.instances = append(u.instances, instance)
}

func (u *unorderedScheduler) getInstances() []*brbInstance {
	return u.instances
}

func (u *unorderedScheduler) stop() {
	u.ticker.Stop()
}

func instantiateCorrect(t *testing.T, outputChans []chan []byte, scheduler scheduler, n, f uint) {
	for _, o := range outputChans {
		echoChan, readyChan := scheduler.getChannels(t, uuid.New())
		instance := newBrbInstance(n, f, echoChan, readyChan, o)
		scheduler.addInstance(instance)
	}
}
