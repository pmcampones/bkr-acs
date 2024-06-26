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

// Please forgive me reader, I know this is not orthodox and not idiomatic at all.
// Slices are passed by value and the append function is the only way to modify them.
// This is a workaround to allow the BCBChannel to have a slice of observers that can be modified across copies of the struct.
// This could have been avoided if the function implementations received pointers and not struct copies, however this is not possible because of the NodeObserver interface.
// It was either this or use a map with the observer as the key. Considering we mostly do iteration on the observers, I chose this.
type sliceWrapper[T any] struct {
	slice []T
}

func (sw *sliceWrapper[T]) get() []T {
	return sw.slice
}

func (sw *sliceWrapper[T]) set(slice []T) {
	sw.slice = slice
}

type BCBChannel struct {
	instances map[UUID]*bcbInstance
	finished  map[UUID]bool
	n         uint
	f         uint
	observers *sliceWrapper[BCBObserver]
	network   *network.Node
}

func (channel BCBChannel) bcbInstanceDeliver(id UUID, msg []byte) {
	for _, observer := range (*channel.observers).get() {
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
	wrapper := sliceWrapper[BCBObserver]{observers}
	channel := BCBChannel{
		instances: make(map[UUID]*bcbInstance),
		finished:  make(map[UUID]bool),
		n:         n,
		f:         f,
		network:   node,
		observers: &wrapper,
	}
	node.AddObserver(channel)
	return &channel
}

func (channel BCBChannel) AttachObserver(observer BCBObserver) {
	(*channel.observers).set(append((*channel.observers).get(), observer))
}

func (channel BCBChannel) BCBroadcast(msg []byte) error {
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
func (channel BCBChannel) BEBDeliver(msg []byte) {
	if bcastType(msg[0]) == bcbMsg {
		channel.processMsg(msg[1:])
	}
}

func (channel BCBChannel) processMsg(msg []byte) {
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
