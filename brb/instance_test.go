package brb

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/samber/lo"
	"testing"
)

type honestDummyMiddleware struct {
	instances []*brbInstance
}

func newHonestDummyMiddleware() *honestDummyMiddleware {
	return &honestDummyMiddleware{instances: make([]*brbInstance, 0)}
}

func (h *honestDummyMiddleware) getChannels(sender uuid.UUID) (chan []byte, chan []byte) {
	echoChan := make(chan []byte)
	readyChan := make(chan []byte)
	go func() {
		msg := <-echoChan
		for _, inst := range h.instances {
			inst.handleEcho(msg, sender)
		}
	}()
	go func() {
		msg := <-readyChan
		for _, inst := range h.instances {
			inst.handleReady(msg, sender)
		}
	}()
	return echoChan, readyChan
}

func (h *honestDummyMiddleware) sendAll(msg []byte) {
	for _, i := range h.instances {
		i.handleSend(msg)
	}
}

func TestShouldReceiveSelfBroadcast(t *testing.T) {
	dummy := newHonestDummyMiddleware()
	echoChan, readyChan := dummy.getChannels(uuid.New())
	outputChan := make(chan []byte)
	instance := newBrbInstance(1, 0, echoChan, readyChan, outputChan)
	dummy.instances = append(dummy.instances, instance)
	msg := []byte("hello")
	instance.handleSend(msg)
	recov := <-outputChan
	assert.Equal(t, recov, msg)
}

func TestShouldBroadcastToAllNoFaultsOrdered(t *testing.T) {
	numNodes := 10
	dummy := newHonestDummyMiddleware()
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	for _, o := range outputChans {
		echoChan, readyChan := dummy.getChannels(uuid.New())
		instance := newBrbInstance(uint(numNodes), 0, echoChan, readyChan, o)
		dummy.instances = append(dummy.instances, instance)
	}
	msg := []byte("hello")
	dummy.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxFaultsOrdered(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	dummy := newHonestDummyMiddleware()
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	for _, o := range outputChans {
		echoChan, readyChan := dummy.getChannels(uuid.New())
		instance := newBrbInstance(uint(numNodes), uint(f), echoChan, readyChan, o)
		dummy.instances = append(dummy.instances, instance)
	}
	msg := []byte("hello")
	dummy.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}
