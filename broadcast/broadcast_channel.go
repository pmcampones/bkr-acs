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

type BCBChannel struct {
	instances map[UUID]*bcbInstance
	finished  map[UUID]bool
	n         uint
	f         uint
	observers []BCBObserver
	network   *network.Node
	sk        *ecdsa.PrivateKey
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
		sk:        &sk,
	}
	node.AddObserver(channel)
	return channel
}

func (channel *BCBChannel) AttachObserver(observer BCBObserver) {
	slog.Info("attaching observer to bcb channel", "observer", observer)
	channel.observers = append(channel.observers, observer)
}

// BCBroadcast broadcasts a message to all nodes in the network satisfying the Byzantine Consistent Broadcast properties
// This ensures all correct processes that deliver a message, deliver the same message
// It does not ensure the message is delivered by all correct processes. Some may deliver and others don't
// This function follows the Authenticated Echo Broadcast algorithm, where messages are not signed and the broadcast incurs two communication rounds.
func (channel *BCBChannel) BCBroadcast(msg []byte) error {
	nonce := uint32(rand.Int())
	id, err := channel.computeBroadcastId(nonce)
	if err != nil {
		return fmt.Errorf("bcb channel unable to compute broadcast id: %v", err)
	}
	bcbInstance, err := newBcbInstance(id, channel.n, channel.f, channel.network)
	if err != nil {
		return fmt.Errorf("bcb channel unable to create bcb instance during send: %v", err)
	}
	slog.Info("sending bcb broadcast", "id", id, "msg", msg)
	channel.instances[id] = bcbInstance
	bcbInstance.attachObserver(channel)
	err = bcbInstance.bcbSend(nonce, msg)
	if err != nil {
		return fmt.Errorf("bcb channel unable to send message during broadcast: %v", err)
	}
	return nil
}

func (channel *BCBChannel) computeBroadcastId(nonce uint32) (UUID, error) {
	encodedPk, err := crypto.SerializePublicKey(&channel.sk.PublicKey)
	if err != nil {
		return UUID{}, fmt.Errorf("bcb channel unable to serialize public key during broadcast: %v", err)
	}
	buf := bytes.NewBuffer(encodedPk)
	buf.Write(crypto.IntToBytes(nonce))
	id := crypto.BytesToUUID(buf.Bytes())
	return id, nil
}

func (channel *BCBChannel) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	reader := bytes.NewReader(msg)
	id, err := getInstanceId(reader, sender)
	if err != nil {
		slog.Error("unable to get instance id from message during bcb delivery", "error", err)
		return
	}
	bcastTp, err := reader.ReadByte()
	if bcastType(bcastTp) == bcbMsg {
		slog.Debug("received message in bcb channel", "msg", msg)
		channel.processMsg(id, reader)
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
	buf.Write(crypto.IntToBytes(nonce))
	id := crypto.BytesToUUID(buf.Bytes())
	return id, nil
}

func (channel *BCBChannel) processMsg(id UUID, reader *bytes.Reader) {
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
	err := instance.bebReceive(reader)
	if err != nil {
		slog.Error("error handling received message in bcb channel", "error", err)
	}
}
