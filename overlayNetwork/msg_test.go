package overlayNetwork

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestShouldReceiveWhatWasSentInSingleMessage(t *testing.T) {
	msg := []byte("Hello World!!")
	auxTestShouldReceiveWhatWasSent([][]byte{msg}, t)
}

func TestShouldReceiveWhatWasSentInMultipleMessages(t *testing.T) {
	numMsg := 100000
	messages := make([][]byte, numMsg)
	for i := 0; i < numMsg; i++ {
		messages[i] = []byte(fmt.Sprintf("Hello World!!%d", i))
	}
	auxTestShouldReceiveWhatWasSent(messages, t)
}

func TestShouldReceiveMBLongMessage(t *testing.T) {
	mb := 1024 * 1024
	auxTestShouldReceiveLongMessage(mb, t)
}

func TestShouldReceiveGBLongMessage(t *testing.T) {
	gb := 1024 * 1024 * 1024
	auxTestShouldReceiveLongMessage(gb, t)
}

func auxTestShouldReceiveLongMessage(size int, t *testing.T) {
	msg := make([]byte, size)
	for i := 0; i < size; i++ {
		msg[i] = byte(i % 256)
	}
	auxTestShouldReceiveWhatWasSent([][]byte{msg}, t)
}

func auxTestShouldReceiveWhatWasSent(messages [][]byte, t *testing.T) {
	address := "localhost:6000"
	listening := make(chan struct{})
	go func() {
		listener, err := net.Listen("tcp", address)
		assert.NoError(t, err)
		listening <- struct{}{}
		connSend, err := listener.Accept()
		assert.NoError(t, err)
		for _, msg := range messages {
			assert.NoError(t, send(connSend, msg))
		}
	}()
	<-listening
	connReceive, err := net.Dial("tcp", address)
	assert.NoError(t, err)
	for _, msg := range messages {
		received, err := receive(connReceive)
		assert.NoError(t, err)
		assert.True(t, bytes.Equal(msg, received))
	}
}
