package broadcast

import (
	"broadcast_channels/crypto"
	"broadcast_channels/network"
	"fmt"
	. "github.com/google/uuid"
	"os"
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
	echoChannel  chan []byte
}

func newBcbInstance(id UUID, n, f uint, network *network.Node) (*bcbInstance, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to create bcb instance due to error in unmarshaling idBytes")
	}
	echoChannel := make(chan []byte, n)
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

func (b *bcbInstance) bcbSend(msg []byte) {
	toSend := sliceJoin([]byte{byte(bcbMsg)}, b.idBytes, []byte{byte(send)}, msg)
	b.network.Broadcast(toSend)
}

func (b *bcbInstance) bebReceive(msg []byte) error {
	msgType := bcb(msg[0])
	switch msgType {
	case send:
		return b.handleSend(msg[1:])
	case echo:
		b.echoChannel <- msg[1:]
		return nil
	default:
		return fmt.Errorf("unhandled default case in received message of opcode %d in bcb instance", msgType)
	}
}

func (b *bcbInstance) handleSend(msg []byte) error {
	if b.sendEcho {
		return fmt.Errorf("already received original message in this instance")
	}
	b.sendEcho = true
	toSend := sliceJoin([]byte{byte(bcbMsg)}, b.idBytes, []byte{byte(echo)}, msg)
	b.network.Broadcast(toSend)
	return nil
}

func (b *bcbInstance) handleEcho(msg []byte) error {
	if b.delivered {
		return fmt.Errorf("already delivered")
	}
	mid := crypto.BytesToUUID(msg)
	b.echos[mid]++
	b.msg[mid] = msg
	threshold := (b.n + b.f) / 2
	if b.echos[mid] > threshold {
		b.delivered = true
		for _, observer := range b.observers {
			observer.bcbInstanceDeliver(b.id, msg)
		}
	}
	return nil
}

func (b *bcbInstance) processEchoes() {
	for echo := range b.echoChannel {
		if b.delivered {
			_, _ = fmt.Fprintf(os.Stderr, "already delivered")
		} else {
			mid := crypto.BytesToUUID(echo)
			b.echos[mid]++
			b.msg[mid] = echo
			threshold := (b.n + b.f) / 2
			if b.echos[mid] > threshold {
				b.delivered = true
				for _, observer := range b.observers {
					observer.bcbInstanceDeliver(b.id, echo)
				}
			}
		}
	}
}

func (b *bcbInstance) close() {
	close(b.echoChannel)
}
