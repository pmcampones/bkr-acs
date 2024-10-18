package brb

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/samber/lo"
	"testing"
)

type honestOrderedScheduler struct {
	instances []*brbInstance
}

func newHonestOrderedScheduler() *honestOrderedScheduler {
	return &honestOrderedScheduler{instances: make([]*brbInstance, 0)}
}

func (h *honestOrderedScheduler) getChannels(t *testing.T, sender uuid.UUID) (chan []byte, chan []byte) {
	echoChan := make(chan []byte)
	readyChan := make(chan []byte)
	go func() {
		msg := <-echoChan
		for _, inst := range h.instances {
			err := inst.handleEcho(msg, sender)
			assert.NoError(t, err)
		}
	}()
	go func() {
		msg := <-readyChan
		for _, inst := range h.instances {
			err := inst.handleReady(msg, sender)
			assert.NoError(t, err)
		}
	}()
	return echoChan, readyChan
}

func (h *honestOrderedScheduler) sendAll(msg []byte) {
	for _, i := range h.instances {
		i.handleSend(msg)
	}
}

func TestShouldReceiveSelfBroadcast(t *testing.T) {
	scheduler := newHonestOrderedScheduler()
	echoChan, readyChan := scheduler.getChannels(t, uuid.New())
	outputChan := make(chan []byte)
	instance := newBrbInstance(1, 0, echoChan, readyChan, outputChan)
	scheduler.instances = append(scheduler.instances, instance)
	msg := []byte("hello")
	instance.handleSend(msg)
	recov := <-outputChan
	assert.Equal(t, recov, msg)
}

func TestShouldBroadcastToAllNoFaultsOrdered(t *testing.T) {
	numNodes := 10
	scheduler := newHonestOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	for _, o := range outputChans {
		echoChan, readyChan := scheduler.getChannels(t, uuid.New())
		instance := newBrbInstance(uint(numNodes), 0, echoChan, readyChan, o)
		scheduler.instances = append(scheduler.instances, instance)
	}
	msg := []byte("hello")
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxFaultsOrdered(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	scheduler := newHonestOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	for _, o := range outputChans {
		echoChan, readyChan := scheduler.getChannels(t, uuid.New())
		instance := newBrbInstance(uint(numNodes), uint(f), echoChan, readyChan, o)
		scheduler.instances = append(scheduler.instances, instance)
	}
	msg := []byte("hello")
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxCrashOrdered(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	scheduler := newHonestOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	for _, o := range outputChans {
		echoChan, readyChan := scheduler.getChannels(t, uuid.New())
		instance := newBrbInstance(uint(numNodes-f), uint(f), echoChan, readyChan, o)
		scheduler.instances = append(scheduler.instances, instance)
	}
	msg := []byte("hello")
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}
