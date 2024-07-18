package network

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"net"
	"testing"
)

func TestShouldReceiveWhatWasSentInSingleMessage(t *testing.T) {
	msg := []byte("Hello World!!")
	address := "localhost:6000"
	listening := make(chan struct{})
	go func() {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Errorf("unable to listen on address: %v", err)
			return
		}
		listening <- struct{}{}
		connSend, err := listener.Accept()
		err = send(connSend, msg)
		if err != nil {
			t.Errorf("unable to send message: %v", err)
			return
		}
	}()
	<-listening
	connReceive, err := net.Dial("tcp", address)
	if err != nil {
		t.Errorf("unable to dial: %v", err)
		return
	}
	received, err := receive(connReceive)
	if err != nil {
		t.Errorf("unable to receive message: %v", err)
		return
	}
	assert.Equal(t, received, msg)
}

func TestShouldReceiveWhatWasSentInMultipleMessages(t *testing.T) {
	address := "localhost:6000"
	numMsg := 100000
	messages := make([][]byte, numMsg)
	for i := 0; i < numMsg; i++ {
		messages[i] = []byte(fmt.Sprintf("Hello World!!%d", i))
	}
	listening := make(chan struct{})
	go func() {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Errorf("unable to listen on address: %v", err)
			return
		}
		listening <- struct{}{}
		connSend, err := listener.Accept()
		for _, msg := range messages {
			err = send(connSend, msg)
			if err != nil {
				t.Errorf("unable to send message: %v", err)
				return
			}
		}
	}()
	<-listening
	connReceive, err := net.Dial("tcp", address)
	if err != nil {
		t.Errorf("unable to dial: %v", err)
		return
	}
	for _, msg := range messages {
		received, err := receive(connReceive)
		if err != nil {
			t.Errorf("unable to receive message: %v", err)
			return
		}
		assert.Equal(t, string(received), string(msg))
	}
}

func TestShouldReceiveMBLongMessage(t *testing.T) {
	mb := 1024 * 1024
	msg := make([]byte, mb)
	for i := 0; i < mb; i++ {
		msg[i] = byte(i % 256)
	}
	address := "localhost:6000"
	listening := make(chan struct{})
	go func() {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Errorf("unable to listen on address: %v", err)
			return
		}
		listening <- struct{}{}
		connSend, err := listener.Accept()
		err = send(connSend, msg)
		if err != nil {
			t.Errorf("unable to send message: %v", err)
			return
		}
	}()
	<-listening
	connReceive, err := net.Dial("tcp", address)
	if err != nil {
		t.Errorf("unable to dial: %v", err)
		return
	}
	received, err := receive(connReceive)
	if err != nil {
		t.Errorf("unable to receive message: %v", err)
		return
	}
	assert.Equal(t, received, msg)
}

func TestShouldReceiveGBLongMessage(t *testing.T) {
	mb := 1024 * 1024 * 1024
	msg := make([]byte, mb)
	for i := 0; i < mb; i++ {
		msg[i] = byte(i % 256)
	}
	address := "localhost:6000"
	listening := make(chan struct{})
	go func() {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Errorf("unable to listen on address: %v", err)
			return
		}
		listening <- struct{}{}
		connSend, err := listener.Accept()
		err = send(connSend, msg)
		if err != nil {
			t.Errorf("unable to send message: %v", err)
			return
		}
	}()
	<-listening
	connReceive, err := net.Dial("tcp", address)
	if err != nil {
		t.Errorf("unable to dial: %v", err)
		return
	}
	received, err := receive(connReceive)
	if err != nil {
		t.Errorf("unable to receive message: %v", err)
		return
	}
	assert.Equal(t, received, msg)
}
