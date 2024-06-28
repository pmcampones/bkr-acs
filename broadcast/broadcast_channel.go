package broadcast

import (
	"broadcast_channels/network"
	"crypto/ecdsa"
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	"unsafe"
)

type bcastType byte

const (
	bcbMsg bcastType = 'A' + iota
	brbMsg
)

type BCBObserver interface {
	BCBDeliver(msg []byte)
}

type BCBChannel struct {
	instances map[UUID]*bcbInstance
	finished  map[UUID]bool
	n         uint
	f         uint
	observers []BCBObserver
	network   *network.Node
	sk        ecdsa.PrivateKey
}

func (channel *BCBChannel) bcbInstanceDeliver(id UUID, msg []byte) {
	for _, observer := range channel.observers {
		observer.BCBDeliver(msg)
	}
	instance, ok := channel.instances[id]
	if !ok {
		slog.Error("bcb channel instance not found upon delivery", "Id", id)
	}
	instance.close()
	delete(channel.instances, id)
	channel.finished[id] = true
}

func BCBCreateChannel(node *network.Node, n, f uint, sk ecdsa.PrivateKey) *BCBChannel {
	observers := make([]BCBObserver, 0)
	channel := &BCBChannel{
		instances: make(map[UUID]*bcbInstance),
		finished:  make(map[UUID]bool),
		n:         n,
		f:         f,
		network:   node,
		observers: observers,
		sk:        sk,
	}
	node.AddObserver(channel)
	return channel
}

func (channel *BCBChannel) AttachObserver(observer BCBObserver) {
	slog.Info("attaching observer to bcb channel", "observer", observer)
	channel.observers = append(channel.observers, observer)
}

func (channel *BCBChannel) BCBroadcast(msg []byte) error {
	id := New()
	bcbInstance, err := newBcbInstance(id, channel.n, channel.f, channel.network)
	if err != nil {
		return fmt.Errorf("bcb channel unable to create bcb instance during send: %v", err)
	}
	slog.Info("sending bcb broadcast", "id", id, "msg", msg)
	channel.instances[id] = bcbInstance
	bcbInstance.attachObserver(channel)
	bcbInstance.bcbSend(msg)
	return nil
}

func (channel *BCBChannel) BEBDeliver(msg []byte) {
	if bcastType(msg[0]) == bcbMsg {
		slog.Debug("received message in bcb channel", "msg", msg)
		channel.processMsg(msg[1:])
	}
}

func (channel *BCBChannel) processMsg(msg []byte) {
	idLen := unsafe.Sizeof(UUID{})
	id := UUID(msg[:idLen])
	if channel.finished[id] {
		slog.Debug("received message from finished instance", "id", id)
		return
	}
	instance, ok := channel.instances[id]
	if !ok {
		var err error // Declare err here to avoid shadowing the instance variable
		instance, err = newBcbInstance(id, channel.n, channel.f, channel.network)
		if err != nil {
			slog.Error("unable to create new bcb instance upon receiving a message", "id", id, "error", err)
			return
		}
		channel.instances[id] = instance
		instance.attachObserver(channel)
	}
	err := instance.bebReceive(msg[idLen:])
	if err != nil {
		slog.Error("error handling received message in bcb channel", "error", err)
	}
}

func sliceJoin(slices ...[]byte) []byte {
	length := 0
	for _, aSlice := range slices {
		length += len(aSlice)
	}
	res := make([]byte, 0, length)
	for _, aSlice := range slices {
		res = append(res, aSlice...)
	}
	return res
}
