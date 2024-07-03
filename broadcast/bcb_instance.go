package broadcast

import (
	"broadcast_channels/crypto"
	"broadcast_channels/network"
	"bytes"
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
	send bcb = 'a' + iota
	echo
)

type bcbInstance struct {
	id           UUID
	idBytes      []byte
	n            uint
	f            uint
	receivedSend bool
	sendEcho     bool
	delivered    bool
	echos        map[UUID]uint
	msg          map[UUID][]byte
	network      *network.Node
	observers    []bcbInstanceObserver
	echoChannel  chan *bytes.Reader
}

func newBcbInstance(id UUID, n, f uint, network *network.Node) (*bcbInstance, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to create bcb instance due to error in unmarshaling idBytes")
	}
	echoChannel := make(chan *bytes.Reader, n)
	instance := &bcbInstance{
		id:          id,
		idBytes:     idBytes,
		n:           n,
		f:           f,
		echos:       make(map[UUID]uint),
		msg:         make(map[UUID][]byte),
		network:     network,
		observers:   make([]bcbInstanceObserver, 0, 1),
		echoChannel: echoChannel,
	}
	go instance.processEchoes()
	return instance, nil
}

func (b *bcbInstance) attachObserver(observer bcbInstanceObserver) {
	b.observers = append(b.observers, observer)
}

func (b *bcbInstance) bcbSend(nonce uint32, msg []byte) error {
	buf := bytes.NewBuffer([]byte{})
	buf.Write([]byte{byte(genId)})
	err := binary.Write(buf, binary.LittleEndian, nonce)
	if err != nil {
		return fmt.Errorf("unable to write nonce to buffer during bcb send: %v", err)
	}
	buf.Write([]byte{byte(bcbMsg)})
	buf.Write([]byte{byte(send)})
	buf.Write(msg)
	b.network.Broadcast(buf.Bytes())
	return nil
}

func (b *bcbInstance) bebReceive(reader *bytes.Reader) error {
	typeByte, err := reader.ReadByte()
	if err != nil {
		return fmt.Errorf("unable to read type byte from message during beb receive: %v", err)
	}
	msgType := bcb(typeByte)
	switch msgType {
	case send:
		return b.handleSend(reader)
	case echo:
		b.echoChannel <- reader
		return nil
	default:
		return fmt.Errorf("unhandled default case in received message of opcode %d in bcb instance", msgType)
	}
}

func (b *bcbInstance) handleSend(reader *bytes.Reader) error {
	if b.sendEcho {
		return fmt.Errorf("already received original message in this instance")
	}
	msg := make([]byte, reader.Len())
	num, err := reader.Read(msg)
	if err != nil {
		return fmt.Errorf("unable to read message from reader during handle send: %v", err)
	} else if num != len(msg) {
		return fmt.Errorf("unable to read message from reader during handle send: read %d bytes, expected %d", num, len(msg))
	}
	b.sendEcho = true
	buf := bytes.NewBuffer([]byte{})
	buf.Write([]byte{byte(withId)})
	buf.Write(b.idBytes)
	buf.Write([]byte{byte(bcbMsg)})
	buf.Write([]byte{byte(echo)})
	buf.Write(msg)
	b.network.Broadcast(buf.Bytes())
	return nil
}

func (b *bcbInstance) processEchoes() {
	for echo := range b.echoChannel {
		if b.delivered {
			slog.Debug("already delivered on BCB instance", "Id", b.id)
		} else {
			msg := make([]byte, echo.Len())
			num, err := echo.Read(msg)
			if err != nil {
				slog.Error("unable to read message from echo during bcb instance processing", "error", err)
				continue
			} else if num != len(msg) {
				slog.Error("unable to read message from echo during bcb instance processing", "read", num, "expected", len(msg))
				continue
			}
			mid := crypto.BytesToUUID(msg)
			slog.Debug("processing echo", "Id", b.id, "Echo", echo, "Content id", mid)
			b.echos[mid]++
			b.msg[mid] = msg
			threshold := (b.n + b.f) / 2
			if b.echos[mid] > threshold {
				b.delivered = true
				for _, observer := range b.observers {
					observer.bcbInstanceDeliver(b.id, msg)
				}
			}
		}
	}
}

func (b *bcbInstance) close() {
	close(b.echoChannel)
}
