package brb

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
	"math/rand"
	"pace/network"
	"pace/utils"
	"reflect"
)

type idHandling byte

const (
	genId idHandling = 'a' + iota
	withId
)

type BRBObserver interface {
	BRBDeliver(msg []byte)
}

type broadcastInstanceObserver interface {
	instanceDeliver(id UUID, msg []byte)
}

type BRBChannel struct {
	instances  map[UUID]*brbInstance
	finished   map[UUID]bool
	n          uint
	f          uint
	observers  []BRBObserver
	network    *network.Node
	sk         *ecdsa.PrivateKey
	commands   chan<- func() error
	listenCode byte
}

func CreateBRBChannel(node *network.Node, n, f uint, sk ecdsa.PrivateKey, listenCode byte) *BRBChannel {
	commands := make(chan func() error)
	channel := &BRBChannel{
		instances:  make(map[UUID]*brbInstance),
		finished:   make(map[UUID]bool),
		n:          n,
		f:          f,
		network:    node,
		observers:  make([]BRBObserver, 0),
		sk:         &sk,
		commands:   commands,
		listenCode: listenCode,
	}
	node.AttachMessageObserver(channel)
	go invoker(commands)
	return channel
}

func (channel *BRBChannel) AttachObserver(observer BRBObserver) {
	slog.Info("attaching observer to bcb channel", "observer", observer)
	channel.observers = append(channel.observers, observer)
}

func (channel *BRBChannel) BRBroadcast(msg []byte) error {
	nonce := rand.Uint32()
	id, err := channel.computeBroadcastId(nonce)
	if err != nil {
		return fmt.Errorf("channel unable to compute brb id: %v", err)
	}
	brbInstance, err := newBrbInstance(id, channel.n, channel.f, channel.network, channel.listenCode)
	if err != nil {
		return fmt.Errorf("channel unable to create bcb instance during bcbsend: %v", err)
	}
	channel.broadcast(msg, nonce, id, brbInstance)
	return nil
}

func (channel *BRBChannel) broadcast(msg []byte, nonce uint32, id UUID, instance *brbInstance) {
	channel.commands <- func() error {
		slog.Info("sending brb", "id", id, "msg", msg)
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

func (channel *BRBChannel) computeBroadcastId(nonce uint32) (UUID, error) {
	encodedPk, err := utils.SerializePublicKey(&channel.sk.PublicKey)
	if err != nil {
		return UUID{}, fmt.Errorf("bcb channel unable to serialize public key during brb: %v", err)
	}
	buf := bytes.NewBuffer(encodedPk)
	err = binary.Write(buf, binary.LittleEndian, nonce)
	if err != nil {
		return UUID{}, fmt.Errorf("unable to write nonce to buffer during brb: %v", err)
	}
	id := utils.BytesToUUID(buf.Bytes())
	return id, nil
}

func (channel *BRBChannel) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	if msg[0] == channel.listenCode {
		slog.Debug("received message from network", "sender", sender)
		msg = msg[1:]
		reader := bytes.NewReader(msg)
		id, err := getInstanceId(reader, sender)
		if err != nil {
			slog.Error("unable to get instance id from message during bcb delivery", "error", err)
		} else {
			channel.commands <- func() error {
				slog.Debug("processing message", "id", id)
				err = channel.processMsg(id, reader, sender)
				if err != nil {
					return fmt.Errorf("unable to process message during bcb delivery: %v", err)
				}
				return nil
			}
		}
	} else {
		slog.Debug("received message was not for me", "sender", sender, "code", msg[0])
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
		return utils.ExtractIdFromMessage(reader)
	default:
		return Nil, fmt.Errorf("unhandled default case in instance id computation")
	}
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
	encodedPk, err := utils.SerializePublicKey(sender)
	if err != nil {
		return Nil, fmt.Errorf("unable to serialize public key during brb: %v", err)
	}
	buf := bytes.NewBuffer(encodedPk)
	err = binary.Write(buf, binary.LittleEndian, nonce)
	if err != nil {
		return UUID{}, fmt.Errorf("unable to write nonce to buffer: %v", err)
	}
	id := utils.BytesToUUID(buf.Bytes())
	return id, nil
}

func (channel *BRBChannel) processMsg(id UUID, reader *bytes.Reader, sender *ecdsa.PublicKey) error {
	if channel.finished[id] {
		slog.Debug("received message from finished instance", "id", id)
		return nil
	}
	instance, ok := channel.instances[id]
	if !ok {
		var err error // Declare err here to avoid shadowing the instance variable
		instance, err = newBrbInstance(id, channel.n, channel.f, channel.network, channel.listenCode)
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

func (channel *BRBChannel) instanceDeliver(id UUID, msg []byte) {
	slog.Debug("delivering message from brb instance", "id", id)
	channel.commands <- func() error {
		for _, observer := range channel.observers {
			observer.BRBDeliver(msg)
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
