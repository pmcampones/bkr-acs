package brb

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
	testShouldBroadcastToAllNoFaults(t, newOrderedScheduler())
}

func TestShouldBroadcastToAllNoFaultsUnordered(t *testing.T) {
	s := newUnorderedScheduler()
	testShouldBroadcastToAllNoFaults(t, s)
	s.stop()
}

func testShouldBroadcastToAllNoFaults(t *testing.T, s scheduler) {
	numNodes := 10
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), 0)
	msg := []byte("hello")
	s.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxFaultsOrdered(t *testing.T) {
	testShouldBroadcastToAllMaxFaults(t, newOrderedScheduler())
}

func TestShouldBroadcastToAllMaxFaultsUnordered(t *testing.T) {
	s := newUnorderedScheduler()
	testShouldBroadcastToAllMaxFaults(t, s)
	s.stop()
}

func testShouldBroadcastToAllMaxFaults(t *testing.T, s scheduler) {
	f := 3
	numNodes := 3*f + 1
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	s.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxCrashOrdered(t *testing.T) {
	testShouldBroadcastToAllMaxCrash(t, newOrderedScheduler())
}

func TestShouldBroadcastToAllMaxCrashUnordered(t *testing.T) {
	s := newUnorderedScheduler()
	testShouldBroadcastToAllMaxCrash(t, s)
	s.stop()
}

func testShouldBroadcastToAllMaxCrash(t *testing.T, s scheduler) {
	f := 3
	numNodes := 3*f + 1
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	s.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldBroadcastToAllMaxByzantineOrdered(t *testing.T) {
	testShouldBroadcastToAllMaxByzantine(t, newOrderedScheduler())
}

func TestShouldBroadcastToAllMaxByzantineUnordered(t *testing.T) {
	s := newUnorderedScheduler()
	testShouldBroadcastToAllMaxByzantine(t, s)
	s.stop()
}

func testShouldBroadcastToAllMaxByzantine(t *testing.T, s scheduler) {
	f := 3
	numNodes := 3*f + 1
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := lo.Map(lo.Range(f), func(_ int, _ int) uuid.UUID { return uuid.New() })
	for _, correct := range s.getInstances() {
		for _, byz := range byzantine {
			err := correct.ready(byzMsg, byz)
			assert.NoError(t, err)
			err = correct.echo(byzMsg, byz)
			assert.NoError(t, err)
		}
	}
	s.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldDetectRepeatedEchoesOrdered(t *testing.T) {
	testShouldDetectRepeatedEchoes(t, newOrderedScheduler())
}

func TestShouldDetectRepeatedEchoesUnordered(t *testing.T) {
	s := newUnorderedScheduler()
	testShouldDetectRepeatedEchoes(t, s)
	s.stop()
}

func testShouldDetectRepeatedEchoes(t *testing.T, s scheduler) {
	f := 3
	numNodes := 3*f + 1
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := uuid.New()
	for _, correct := range s.getInstances() {
		err := correct.echo(byzMsg, byzantine)
		assert.NoError(t, err)
		err = correct.echo(byzMsg, byzantine)
		assert.True(t, err != nil)
	}
	s.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}

func TestShouldDetectRepeatedReadiesOrdered(t *testing.T) {
	testShouldDetectRepeatedReadies(t, newOrderedScheduler())
}

func TestShouldDetectRepeatedReadiesUnordered(t *testing.T) {
	s := newUnorderedScheduler()
	testShouldDetectRepeatedReadies(t, s)
	s.stop()
}

func testShouldDetectRepeatedReadies(t *testing.T, s scheduler) {
	f := 3
	numNodes := 3*f + 1
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan []byte { return make(chan []byte) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := uuid.New()
	for _, correct := range s.getInstances() {
		err := correct.ready(byzMsg, byzantine)
		assert.NoError(t, err)
		err = correct.ready(byzMsg, byzantine)
		assert.True(t, err != nil)
	}
	s.sendAll(msg)
	outputs := lo.Map(outputChans, func(o chan []byte, _ int) []byte { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov []byte) bool { return bytes.Equal(recov, msg) }))
}
