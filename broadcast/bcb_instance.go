package broadcast

import (
	"broadcast_channels/crypto"
	"broadcast_channels/network"
	"fmt"
	. "github.com/google/uuid"
	"github.com/samber/mo"
)

type bcb byte

const (
	send bcb = 'a' + iota
	echo
)

type bcbInstance struct {
	id           []byte
	n            uint
	f            uint
	receivedSend bool
	sendEcho     bool
	delivered    bool
	echos        map[UUID]uint
	msg          map[UUID][]byte
	network      *network.Node
}

func newBcbInstance(id UUID, n, f uint, network *network.Node) (*bcbInstance, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to create bcb instance due to error in unmarshaling id")
	}
	instance := &bcbInstance{
		id:      idBytes,
		n:       n,
		f:       f,
		echos:   make(map[UUID]uint),
		msg:     make(map[UUID][]byte),
		network: network,
	}
	return instance, nil
}

func (b *bcbInstance) bcbSend(msg []byte) {
	toSend := sliceJoin([]byte{byte(bcbMsg)}, b.id, []byte{byte(send)}, msg)
	b.network.Broadcast(toSend)
}

func (b *bcbInstance) bebReceive(msg []byte) (mo.Option[[]byte], error) {
	msgType := bcb(msg[0])
	switch msgType {
	case send:
		return mo.None[[]byte](), b.handleSend(msg[1:])
	case echo:
		return b.handleEcho(msg[1:])
	default:
		return mo.None[[]byte](), fmt.Errorf("unhandled default case in received message of opcode %d in bcb instance", msgType)
	}
}

func (b *bcbInstance) handleSend(msg []byte) error {
	if b.sendEcho {
		return fmt.Errorf("already received original message in this instance")
	}
	b.sendEcho = true
	toSend := sliceJoin([]byte{byte(bcbMsg)}, b.id, []byte{byte(echo)}, msg)
	b.network.Broadcast(toSend)
	return nil
}

func (b *bcbInstance) handleEcho(msg []byte) (mo.Option[[]byte], error) {
	if b.delivered {
		return mo.None[[]byte](), fmt.Errorf("already delivered")
	}
	mid := crypto.BytesToUUID(msg)
	b.echos[mid]++
	b.msg[mid] = msg
	threshold := (b.n + b.f) / 2
	if b.echos[mid] > threshold {
		b.delivered = true
		return mo.Some[[]byte](msg), nil
	} else {
		return mo.None[[]byte](), nil
	}
}
