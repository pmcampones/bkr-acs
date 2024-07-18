package broadcast

import (
	"broadcast_channels/crypto"
	"broadcast_channels/network"
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	"math/rand"
	"reflect"
	"unsafe"
)

type bcastType byte

type idHandling byte

const (
	bcbMsg bcastType = 'A' + iota
	brbMsg
)

const (
	genId idHandling = 'a' + iota
	withId
)

type BCBObserver interface {
	BCBDeliver(msg []byte)
}

type broadcastInstanceObserver interface {
	instanceDeliver(id UUID, msg []byte)
}

type broadcastInstance interface {
	attachObserver(observer broadcastInstanceObserver)
	handleMessage(reader *bytes.Reader, sender *ecdsa.PublicKey) error
	send(nonce uint32, msg []byte) error
	close()
}

type Channel struct {
	instances map[UUID]broadcastInstance
	finished  map[UUID]bool
	n         uint
	f         uint
	observers []BCBObserver
	network   *network.Node
	sk        *ecdsa.PrivateKey
	commands  chan<- func() error
}

func CreateChannel(node *network.Node, n, f uint, sk ecdsa.PrivateKey) *Channel {
	observers := make([]BCBObserver, 0)
	commands := make(chan func() error)
	channel := &Channel{
		instances: make(map[UUID]broadcastInstance),
		finished:  make(map[UUID]bool),
		n:         n,
		f:         f,
		network:   node,
		observers: observers,
		sk:        &sk,
		commands:  commands,
	}
	node.AddObserver(channel)
	go invoker(commands)
	return channel
}

func (channel *Channel) AttachObserver(observer BCBObserver) {
	slog.Info("attaching observer to bcb channel", "observer", observer)
	channel.observers = append(channel.observers, observer)
}

// BCBroadcast broadcasts a message to all nodes in the network satisfying the Byzantine Consistent Broadcast properties
// This ensures all correct processes that deliver a message, deliver the same message
// It does not ensure the message is delivered by all correct processes. Some may deliver and others don't
// This function follows the Authenticated Echo Broadcast algorithm, where messages are not signed and the broadcast incurs two communication rounds.
func (channel *Channel) BCBroadcast(msg []byte) error {
	nonce := rand.Uint32()
	id, err := channel.computeBroadcastId(nonce)
	if err != nil {
		return fmt.Errorf("channel unable to compute broadcast id: %v", err)
	}
	bcbInstance, err := newBcbInstance(id, channel.n, channel.f, channel.network)
	if err != nil {
		return fmt.Errorf("bcb channel unable to create bcb instance during bcbsend: %v", err)
	}
	channel.broadcast(msg, nonce, id, bcbInstance)
	return nil
}

func (channel *Channel) BRBroadcast(msg []byte) error {
	nonce := rand.Uint32()
	id, err := channel.computeBroadcastId(nonce)
	if err != nil {
		return fmt.Errorf("channel unable to compute broadcast id: %v", err)
	}
	brbInstance, err := newBrbInstance(id, channel.n, channel.f, channel.network)
	if err != nil {
		return fmt.Errorf("channel unable to create bcb instance during bcbsend: %v", err)
	}
	channel.broadcast(msg, nonce, id, brbInstance)
	return nil
}

func (channel *Channel) broadcast(msg []byte, nonce uint32, id UUID, instance broadcastInstance) {
	channel.commands <- func() error {
		slog.Info("sending broadcast", "id", id, "msg", msg)
		channel.instances[id] = instance
		instance.attachObserver(channel)
		go func() {
			err := instance.send(nonce, msg)
			if err != nil {
				slog.Error("unable to send message", "id", id, "type", reflect.TypeOf(instance))
			}
		}()
		return nil
	}
}

func (channel *Channel) computeBroadcastId(nonce uint32) (UUID, error) {
	encodedPk, err := crypto.SerializePublicKey(&channel.sk.PublicKey)
	if err != nil {
		return UUID{}, fmt.Errorf("bcb channel unable to serialize public key during broadcast: %v", err)
	}
	buf := bytes.NewBuffer(encodedPk)
	err = binary.Write(buf, binary.LittleEndian, nonce)
	if err != nil {
		return UUID{}, fmt.Errorf("unable to write nonce to buffer during broadcast: %v", err)
	}
	id := crypto.BytesToUUID(buf.Bytes())
	return id, nil
}

func (channel *Channel) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	slog.Debug("received message from network", "sender", sender)
	channel.commands <- func() error {
		reader := bytes.NewReader(msg)
		id, err := getInstanceId(reader, sender)
		if err != nil {
			return fmt.Errorf("unable to get instance id from message during bcb delivery: %v", err)
		}
		bType, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("unable to read broadcast type from message during bcb delivery: %v", err)
		}
		slog.Debug("processing message", "id", id)
		err = channel.processMsg(id, bcastType(bType), reader, sender)
		if err != nil {
			return fmt.Errorf("unable to process message during bcb delivery: %v", err)
		}
		return nil
	}
}

func getInstanceId(reader *bytes.Reader, sender *ecdsa.PublicKey) (UUID, error) {
	byteType, err := reader.ReadByte()
	if err != nil {
		return Nil, fmt.Errorf("unable to read byte type from message during instance id computation: %v", err)
	}
	switch idHandling(byteType) {
	case genId:
		return processIdGeneration(reader, sender)
	case withId:
		return extractIdFromMessage(reader)
	default:
		return Nil, fmt.Errorf("unhandled default case in instance id computation")
	}
}

func extractIdFromMessage(reader *bytes.Reader) (UUID, error) {
	idLen := unsafe.Sizeof(UUID{})
	idBytes := make([]byte, idLen)
	num, err := reader.Read(idBytes)
	if err != nil {
		return Nil, fmt.Errorf("unable to read idBytes from message during instance idBytes computation: %v", err)
	} else if num != int(idLen) {
		return Nil, fmt.Errorf("unable to read idBytes from message during instance idBytes computation: read %d bytes, expected %d", num, idLen)
	}
	id, err := FromBytes(idBytes)
	if err != nil {
		return Nil, fmt.Errorf("unable to convert idBytes to UUID during instance idBytes computation: %v", err)
	}
	return id, nil
}

func processIdGeneration(reader *bytes.Reader, sender *ecdsa.PublicKey) (UUID, error) {
	var nonce uint32
	err := binary.Read(reader, binary.LittleEndian, &nonce)
	if err != nil {
		return UUID{}, fmt.Errorf("unable to read nonce from message during instance id computation: %v", err)
	}
	id, err := computeInstanceId(nonce, sender)
	if err != nil {
		return Nil, fmt.Errorf("unable to compute instance id on received message: %v", err)
	}
	return id, nil
}

func computeInstanceId(nonce uint32, sender *ecdsa.PublicKey) (UUID, error) {
	encodedPk, err := crypto.SerializePublicKey(sender)
	if err != nil {
		return Nil, fmt.Errorf("unable to serialize public key during broadcast: %v", err)
	}
	buf := bytes.NewBuffer(encodedPk)
	err = binary.Write(buf, binary.LittleEndian, nonce)
	if err != nil {
		return UUID{}, fmt.Errorf("unable to write nonce to buffer: %v", err)
	}
	id := crypto.BytesToUUID(buf.Bytes())
	return id, nil
}

func (channel *Channel) processMsg(id UUID, bType bcastType, reader *bytes.Reader, sender *ecdsa.PublicKey) error {
	if channel.finished[id] {
		slog.Debug("received message from finished instance", "id", id)
		return nil
	}
	instance, ok := channel.instances[id]
	if !ok {
		var err error // Declare err here to avoid shadowing the instance variable
		// Need to read the first byte to determine whether the instance to be instantiated is BCB or BRB
		switch bType {
		case bcbMsg:
			instance, err = newBcbInstance(id, channel.n, channel.f, channel.network)
		case brbMsg:
			instance, err = newBrbInstance(id, channel.n, channel.f, channel.network)
		default:
			return fmt.Errorf("received unknown broadcast type %v", bType)
		}
		if err != nil {
			return fmt.Errorf("unable to create new instance %s upon receiving a message: %v", id, err)
		}
		channel.instances[id] = instance
		instance.attachObserver(channel)
	}
	go func() {
		err := instance.handleMessage(reader, sender)
		if err != nil {
			slog.Warn("unable to handle received message", "id", id, "sender", sender, "error", err)
		}
	}()
	return nil
}

func (channel *Channel) newBroadcastInstance(id UUID, bType bcastType) (broadcastInstance, error) {
	switch bType {
	case bcbMsg:
		instance, err := newBcbInstance(id, channel.n, channel.f, channel.network)
		if err != nil {
			return nil, fmt.Errorf("unable to create new BCB instance: %v", err)
		}
		return instance, nil
	case brbMsg:
		instance, err := newBrbInstance(id, channel.n, channel.f, channel.network)
		if err != nil {
			return nil, fmt.Errorf("unable to create new BRB instance: %v", err)
		}
		return instance, nil
	default:
		return nil, fmt.Errorf("received unknown broadcast message type")
	}
}

func (channel *Channel) instanceDeliver(id UUID, msg []byte) {
	slog.Debug("delivering message from broadcast instance", "id", id)
	channel.commands <- func() error {
		for _, observer := range channel.observers {
			observer.BCBDeliver(msg)
		}
		instance, ok := channel.instances[id]
		if !ok {
			return fmt.Errorf("channel instance %s not found upon delivery", id)
		}
		channel.finished[id] = true
		delete(channel.instances, id)
		go instance.close()
		return nil
	}
}

func invoker(commands <-chan func() error) {
	for command := range commands {
		err := command()
		if err != nil {
			slog.Error("error executing command", "error", err)
		}
	}
}
