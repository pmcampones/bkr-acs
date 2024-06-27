package broadcast

import (
	"broadcast_channels/network"
	"fmt"
	. "github.com/google/uuid"
	"log"
	"unsafe"
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
}

func (channel *BCBChannel) bcbInstanceDeliver(id UUID, msg []byte) {
	for _, observer := range channel.observers {
		observer.BCBDeliver(msg)
	}
	instance, ok := channel.instances[id]
	if !ok {
		log.Fatalf("bcb channel instance with id %s not found upon delivery\n", id)
	}
	instance.close()
	delete(channel.instances, id)
	channel.finished[id] = true
}

func BCBCreateChannel(node *network.Node, n, f uint) *BCBChannel {
	observers := make([]BCBObserver, 0)
	channel := &BCBChannel{
		instances: make(map[UUID]*bcbInstance),
		finished:  make(map[UUID]bool),
		n:         n,
		f:         f,
		network:   node,
		observers: observers,
	}
	node.AddObserver(channel)
	return channel
}

func (channel *BCBChannel) AttachObserver(observer BCBObserver) {
	channel.observers = append(channel.observers, observer)
}

func (channel *BCBChannel) BCBroadcast(msg []byte) error {
	id := New()
	bcbInstance, err := newBcbInstance(id, channel.n, channel.f, channel.network)
	if err != nil {
		return fmt.Errorf("bcb channel unable to create bcb instance during send: %v", err)
	}
	channel.instances[id] = bcbInstance
	bcbInstance.attachObserver(channel)
	bcbInstance.bcbSend(msg)
	return nil
}

// Can't call this method by reference because it implements NodeObserver.
// It's ok because map is a reference and n and f don't change, however it does mess with the observers slice.
func (channel *BCBChannel) BEBDeliver(msg []byte) {
	if bcastType(msg[0]) == bcbMsg {
		channel.processMsg(msg[1:])
	}
}

func (channel *BCBChannel) processMsg(msg []byte) {
	idLen := unsafe.Sizeof(UUID{})
	id := UUID(msg[:idLen])
	if channel.finished[id] {
		log.Printf("received message from finished instance with id: %s\n", id)
		return
	}
	instance, ok := channel.instances[id]
	if !ok {
		var err error // Declare err here to avoid shadowing the instance variable
		instance, err = newBcbInstance(id, channel.n, channel.f, channel.network)
		if err != nil {
			log.Printf("unable to create new bcb instance with idBytes %s upon receiving a message: %s\n", id, err)
			return
		}
		channel.instances[id] = instance
		instance.attachObserver(channel)
	}
	err := instance.bebReceive(msg[idLen:])
	if err != nil {
		log.Println("error handling received message in bcb channel:", err)
	}
}
