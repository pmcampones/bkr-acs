package broadcast

import (
	"broadcast_channels/crypto"
	"broadcast_channels/network"
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
)

type bcb byte

const (
	bcbsend bcb = 'a' + iota
	bcbecho
)

type bcbInstance struct {
	id            UUID
	idBytes       []byte
	n             uint
	f             uint
	receivedSend  bool
	sentEcho      bool
	delivered     bool
	echos         map[UUID]uint
	peersReceived map[ecdsa.PublicKey]bool
	network       *network.Node
	observers     []broadcastInstanceObserver
	closeChan     chan struct{}
	echoChannel   chan struct {
		*bytes.Reader
		*ecdsa.PublicKey
	}
}

func newBcbInstance(id UUID, n, f uint, network *network.Node) (*bcbInstance, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to create bcb instance due to error in unmarshaling idBytes")
	}
	echoChannel := make(chan struct {
		*bytes.Reader
		*ecdsa.PublicKey
	}, n)
	instance := &bcbInstance{
		id:            id,
		idBytes:       idBytes,
		n:             n,
		f:             f,
		echos:         make(map[UUID]uint),
		peersReceived: make(map[ecdsa.PublicKey]bool),
		network:       network,
		observers:     make([]broadcastInstanceObserver, 0, 1),
		closeChan:     make(chan struct{}),
		echoChannel:   echoChannel,
	}
	go instance.processEchoes()
	return instance, nil
}

func (b *bcbInstance) attachObserver(observer broadcastInstanceObserver) {
	b.observers = append(b.observers, observer)
}

func (b *bcbInstance) send(nonce uint32, msg []byte) error {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	_, err := writer.Write([]byte{byte(genId)})
	if err != nil {
		return fmt.Errorf("unable to write genId to buffer: %v", err)
	}
	err = binary.Write(writer, binary.LittleEndian, nonce)
	if err != nil {
		return fmt.Errorf("unable to write nonce to buffer: %v", err)
	}
	_, err = writer.Write([]byte{byte(bcbMsg)})
	if err != nil {
		return fmt.Errorf("unable to write bcbMsg to buffer: %v", err)
	}
	_, err = writer.Write([]byte{byte(bcbsend)})
	if err != nil {
		return fmt.Errorf("unable to write bcbsend to buffer: %v", err)
	}
	_, err = writer.Write(msg)
	if err != nil {
		return fmt.Errorf("unable to write message to buffer: %v", err)
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("unable to flush buffer: %v", err)
	}
	b.network.Broadcast(buf.Bytes())
	return nil
}

func (b *bcbInstance) handleMessage(reader *bytes.Reader, sender *ecdsa.PublicKey) error {
	typeByte, err := reader.ReadByte()
	if err != nil {
		slog.Error("unable to read type byte from message during beb receive", "error", err)
	}
	msgType := bcb(typeByte)
	switch msgType {
	case bcbsend:
		err := b.handleSend(reader)
		if err != nil {
			slog.Error("unable to handle send message during bcb instance beb receive", "error", err)
		}
	case bcbecho:
		b.echoChannel <- struct {
			*bytes.Reader
			*ecdsa.PublicKey
		}{reader, sender}
	default:
		slog.Error("unhandled default case in received message in bcb instance", "opcode", msgType)
	}
	return nil
}

func (b *bcbInstance) handleSend(reader *bytes.Reader) error {
	if b.sentEcho {
		return fmt.Errorf("already received original message in this instance")
	}
	msg := make([]byte, reader.Len())
	num, err := reader.Read(msg)
	if err != nil {
		return fmt.Errorf("unable to read message from reader during handle bcbsend: %v", err)
	} else if num != len(msg) {
		return fmt.Errorf("unable to read message from reader during handle bcbsend: read %d bytes, expected %d", num, len(msg))
	}
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	_, err = writer.Write([]byte{byte(withId)})
	if err != nil {
		return fmt.Errorf("unable to write withId to buffer during handle bcbsend: %v", err)
	}
	_, err = writer.Write(b.idBytes)
	if err != nil {
		return fmt.Errorf("unable to write broadcast id to buffer during handle bcbsend: %v", err)
	}
	_, err = writer.Write([]byte{byte(bcbMsg)})
	if err != nil {
		return fmt.Errorf("unable to write bcbMsg to buffer during handle bcbsend: %v", err)
	}
	_, err = writer.Write([]byte{byte(bcbecho)})
	if err != nil {
		return fmt.Errorf("unable to write bcbecho to buffer during handle bcbsend: %v", err)
	}
	_, err = writer.Write(msg)
	if err != nil {
		return fmt.Errorf("unable to write message to buffer during handle bcbsend: %v", err)
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("unable to flush buffer during handle bcbsend: %v", err)
	}
	b.network.Broadcast(buf.Bytes())
	b.sentEcho = true
	return nil
}

func (b *bcbInstance) processEchoes() {
	for {
		select {
		case echoPair := <-b.echoChannel:
			b.processEcho(echoPair.Reader, echoPair.PublicKey)
		case <-b.closeChan:
			slog.Debug("Stopped processing echoes", "id", b.id)
			return
		}
	}
}

func (b *bcbInstance) processEcho(echo *bytes.Reader, sender *ecdsa.PublicKey) {
	if b.peersReceived[*sender] {
		slog.Warn("already received echo from peer", "Id", b.id, "sender", *sender)
		return
	} else {
		slog.Debug("received echo from peer", "Id", b.id, "sender", *sender)
		b.peersReceived[*sender] = true
	}
	msg := make([]byte, echo.Len())
	num, err := echo.Read(msg)
	if err != nil {
		slog.Error("unable to read message from bcbecho during bcb instance processing", "error", err)
		return
	} else if num != len(msg) {
		slog.Error("unable to read message from bcbecho during bcb instance processing", "read", num, "expected", len(msg))
		return
	}
	mid := crypto.BytesToUUID(msg)
	slog.Debug("processing bcbecho", "Id", b.id, "Echo", echo, "Content id", mid)
	b.echos[mid]++
	threshold := (b.n + b.f) / 2
	if b.echos[mid] == threshold {
		b.delivered = true
		for _, observer := range b.observers {
			go observer.instanceDeliver(b.id, msg)
		}
	}
}

func (b *bcbInstance) close() {
	slog.Debug("sending signal to close bcb instance", "Id", b.id)
	b.closeChan <- struct{}{}
}
