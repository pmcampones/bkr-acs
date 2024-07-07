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

type bcbInstanceObserver interface {
	bcbInstanceDeliver(id UUID, msg []byte)
}

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
	msg           map[UUID][]byte
	peersReceived map[ecdsa.PublicKey]bool
	network       *network.Node
	observers     []bcbInstanceObserver
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
		msg:           make(map[UUID][]byte),
		peersReceived: make(map[ecdsa.PublicKey]bool),
		network:       network,
		observers:     make([]bcbInstanceObserver, 0, 1),
		echoChannel:   echoChannel,
	}
	go instance.processEchoes()
	return instance, nil
}

func (b *bcbInstance) attachObserver(observer bcbInstanceObserver) {
	b.observers = append(b.observers, observer)
}

func (b *bcbInstance) bcbSend(nonce uint32, msg []byte) {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	_, err := writer.Write([]byte{byte(genId)})
	if err != nil {
		slog.Error("unable to write genId to buffer during bcb bcbsend", "error", err)
		return
	}
	err = binary.Write(writer, binary.LittleEndian, nonce)
	if err != nil {
		slog.Error("unable to write nonce to buffer during bcb bcbsend", "error", err)
		return
	}
	_, err = writer.Write([]byte{byte(bcbMsg)})
	if err != nil {
		slog.Error("unable to write bcbMsg to buffer during bcb bcbsend", "error", err)
		return
	}
	_, err = writer.Write([]byte{byte(bcbsend)})
	if err != nil {
		slog.Error("unable to write bcbsend to buffer during bcb bcbsend", "error", err)
		return
	}
	_, err = writer.Write(msg)
	if err != nil {
		slog.Error("unable to write message to buffer during bcb bcbsend", "error", err)
		return
	}
	err = writer.Flush()
	if err != nil {
		slog.Error("unable to flush buffer during bcb bcbsend", "error", err)
	}
	b.network.Broadcast(buf.Bytes())
}

func (b *bcbInstance) bebReceive(reader *bytes.Reader, sender *ecdsa.PublicKey) {
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
	b.msg[mid] = msg
	threshold := (b.n + b.f) / 2
	if b.echos[mid] == threshold {
		b.delivered = true
		for _, observer := range b.observers {
			go observer.bcbInstanceDeliver(b.id, msg)
		}
	}
}

func (b *bcbInstance) close() {
	slog.Debug("sending signal to close bcb instance", "Id", b.id)
	b.closeChan <- struct{}{}
}
