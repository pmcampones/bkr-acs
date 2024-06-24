package broadcast

import (
	"fmt"
	"fullMembership/crypto"
	"fullMembership/network"
	. "github.com/google/uuid"
	"github.com/samber/mo"
	"log"
	"unsafe"
)

type BCBObserver interface {
	BCBDeliver(msg []byte)
}

type BCBChannel struct {
	instances map[UUID]*bcbInstance
	n         uint
	f         uint
	observers []BCBObserver
	network   *network.Node
}

func BCBCreateChannel(node *network.Node, n, f uint) *BCBChannel {
	channel := BCBChannel{
		instances: make(map[UUID]*bcbInstance),
		n:         n,
		f:         f,
		network:   node,
	}
	node.AddObserver(channel)
	return &channel
}

func (channel BCBChannel) AttachObserver(observer BCBObserver) {
	channel.observers = append(channel.observers, observer)
}

func (channel BCBChannel) BCBroadcast(msg []byte) error {
	id := New()
	bcbInstance, err := newBcbInstance(id, channel.n, channel.f, channel.network)
	if err != nil {
		return fmt.Errorf("bcb channel unable to create bcb instance during send: %v", err)
	}
	channel.instances[id] = bcbInstance
	bcbInstance.bcbSend(msg)
	return nil
}

// Can't call this method by reference because it implements NodeObserver. It's ok because map is a reference and n and f don't change
func (channel BCBChannel) BEBDeliver(msg []byte) {
	if bcastType(msg[0]) == bcbMsg {
		channel.processMsg(msg[1:])
	}
}

func (channel BCBChannel) processMsg(msg []byte) {
	idLen := unsafe.Sizeof(UUID{})
	id := crypto.BytesToUUID(msg[:idLen])
	instance, ok := channel.instances[id]
	if !ok {
		instance, err := newBcbInstance(id, channel.n, channel.f, channel.network)
		if err != nil {
			log.Printf("unable to create new bcb instance with id %s upon receiving a message: %s\n", id, err)
			return
		}
		channel.instances[id] = instance
	}
	receive, err := instance.bebReceive(msg[idLen:])
	if err != nil {
		log.Println("error handling received message in bcb channel:", err)
	}
	if receive.IsPresent() {
		channel.deliverMsg(receive)
	}
}

func (channel BCBChannel) deliverMsg(receive mo.Option[[]byte]) {
	deliveredMsg := receive.MustGet()
	for _, observer := range channel.observers {
		observer.BCBDeliver(deliveredMsg)
	}
}
