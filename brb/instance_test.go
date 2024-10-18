package brb

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/samber/lo"
	"testing"
)

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

func TestShouldReceiveSelfBroadcast(t *testing.T) {
	scheduler := newOrderedScheduler()
	echoChan, readyChan := scheduler.getChannels(t, uuid.New())
	outputChan := make(chan []byte)
	instance := newBrbInstance(1, 0, echoChan, readyChan, outputChan)
	scheduler.instances = append(scheduler.instances, instance)
	msg := []byte("hello")
	instance.send(msg)
	recov := <-outputChan
	assert.Equal(t, recov, msg)
}

func TestShouldBroadcastToAllNoFaultsOrdered(t *testing.T) {
	numNodes := 10
	scheduler := newOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, scheduler, uint(numNodes), 0)
	msg := []byte("hello")
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxFaultsOrdered(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	scheduler := newOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, scheduler, uint(numNodes), uint(f))
	msg := []byte("hello")
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxCrashOrdered(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	scheduler := newOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, scheduler, uint(numNodes), uint(f))
	msg := []byte("hello")
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxByzantineOrdered(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	scheduler := newOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, scheduler, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := lo.Map(lo.Range(f), func(_ int, _ int) uuid.UUID { return uuid.New() })
	for _, correct := range scheduler.instances {
		for _, byz := range byzantine {
			err := correct.ready(byzMsg, byz)
			assert.NoError(t, err)
			err = correct.echo(byzMsg, byz)
			assert.NoError(t, err)
		}
	}
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldDetectRepeatedEchoes(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	scheduler := newOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, scheduler, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := uuid.New()
	for _, correct := range scheduler.instances {
		err := correct.echo(byzMsg, byzantine)
		assert.NoError(t, err)
		err = correct.echo(byzMsg, byzantine)
		assert.True(t, err != nil)
	}
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldDetectRepeatedReadies(t *testing.T) {
	f := 3
	numNodes := 3*f + 1
	scheduler := newOrderedScheduler()
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, scheduler, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := uuid.New()
	for _, correct := range scheduler.instances {
		err := correct.ready(byzMsg, byzantine)
		assert.NoError(t, err)
		err = correct.ready(byzMsg, byzantine)
		assert.True(t, err != nil)
	}
	scheduler.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func instantiateCorrect(t *testing.T, outputChans []chan []byte, scheduler *orderedScheduler, n, f uint) {
	for _, o := range outputChans {
		echoChan, readyChan := scheduler.getChannels(t, uuid.New())
		instance := newBrbInstance(n, f, echoChan, readyChan, o)
		scheduler.instances = append(scheduler.instances, instance)
	}
}
