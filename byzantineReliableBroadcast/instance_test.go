package byzantineReliableBroadcast

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
	outputChan := make(chan BRBMsg)
	instance := newBrbInstance(1, 0, echoChan, readyChan, outputChan)
	scheduler.instances = append(scheduler.instances, instance)
	msg := []byte("hello")
	assert.NoError(t, instance.send(msg, uuid.New()))
	recov := <-outputChan
	assert.Equal(t, msg, recov.Content)
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
	numNodes := 2
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), 0)
	msg := []byte("hello")
	s.sendAll(t, msg)
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(recov.Content, msg) }))
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
	outputChans := lo.Map(lo.Range(numNodes), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	s.sendAll(t, msg)
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(recov.Content, msg) }))
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
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	s.sendAll(t, msg)
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(recov.Content, msg) }))
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
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := lo.Map(lo.Range(f), func(_ int, _ int) uuid.UUID { return uuid.New() })
	for _, correct := range s.getInstances() {
		for _, byz := range byzantine {
			assert.NoError(t, correct.ready(byzMsg, byz))
			assert.NoError(t, correct.echo(byzMsg, byz))
		}
	}
	s.sendAll(t, msg)
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(recov.Content, msg) }))
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
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := uuid.New()
	for _, correct := range s.getInstances() {
		assert.NoError(t, correct.echo(byzMsg, byzantine))
		assert.Error(t, correct.echo(byzMsg, byzantine))
	}
	s.sendAll(t, msg)
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(recov.Content, msg) }))
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
	outputChans := lo.Map(lo.Range(numNodes-f), func(_ int, _ int) chan BRBMsg { return make(chan BRBMsg) })
	instantiateCorrect(t, outputChans, s, uint(numNodes), uint(f))
	msg := []byte("hello")
	byzMsg := []byte("bye")
	byzantine := uuid.New()
	for _, correct := range s.getInstances() {
		assert.NoError(t, correct.ready(byzMsg, byzantine))
		assert.Error(t, correct.ready(byzMsg, byzantine))
	}
	s.sendAll(t, msg)
	outputs := lo.Map(outputChans, func(o chan BRBMsg, _ int) BRBMsg { return <-o })
	assert.True(t, lo.EveryBy(outputs, func(recov BRBMsg) bool { return bytes.Equal(recov.Content, msg) }))
}
