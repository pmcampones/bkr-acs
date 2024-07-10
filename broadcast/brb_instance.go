package broadcast

import (
	"broadcast_channels/crypto"
	"broadcast_channels/network"
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	. "github.com/google/uuid"
	"log/slog"
)

// brbState defines the functionalities required for handling the brb messages depending on the current phase of the algorithm.
// This implementation follows the State pattern.
type brbState interface {
	handleSend(msg []byte) error
	handleEcho(msg []byte, id UUID) error
	handleReady(msg []byte, id UUID) error
}

type brbInstanceObserver interface {
	brbInstanceDeliver(id UUID, msg []byte)
}

type brb byte

const (
	brbsend brb = 'a' + iota
	brbecho
	brbready
)

type brbInstance struct {
	data          *brbData
	peersEchoed   map[ecdsa.PublicKey]bool
	peersReadied  map[ecdsa.PublicKey]bool
	tasks         chan<- func() error
	closeChan     chan<- struct{}
	concreteState brbState
}

type brbData struct {
	id        UUID
	idBytes   []byte
	n         uint
	f         uint
	echoes    map[UUID]uint
	readies   map[UUID]uint
	network   *network.Node
	observers []brbInstanceObserver
}

type msgStruct struct {
	id      UUID
	content []byte
	kind    brb
}

func newBrbInstance(id UUID, n, f uint, network *network.Node) (*brbInstance, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to create brb instance due to error in unmarshaling idBytes")
	}
	data := brbData{
		id:        id,
		idBytes:   idBytes,
		n:         n,
		f:         f,
		echoes:    make(map[UUID]uint),
		readies:   make(map[UUID]uint),
		network:   network,
		observers: make([]brbInstanceObserver, 0, 1),
	}
	ph3 := brbPhase3Handler{
		&data,
	}
	ph2 := brbPhase2Handler{
		data:       &data,
		isFinished: false,
		nextPhase:  &ph3,
	}
	ph1 := brbPhase1Handler{
		data:       &data,
		isFinished: false,
		nextPhase:  &ph2,
	}
	tasks := make(chan func() error)
	closeChan := make(chan struct{})
	instance := &brbInstance{
		data:          &data,
		peersEchoed:   make(map[ecdsa.PublicKey]bool),
		peersReadied:  make(map[ecdsa.PublicKey]bool),
		tasks:         tasks,
		closeChan:     closeChan,
		concreteState: &ph1,
	}
	go instance.executorService(tasks, closeChan)
	return instance, nil
}

func (b *brbInstance) attachObserver(observer brbInstanceObserver) {
	b.data.observers = append(b.data.observers, observer)
}

func (b *brbInstance) brbSend(nonce uint32, msg []byte) error {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	_, err := writer.Write([]byte{byte(genId)})
	if err != nil {
		return fmt.Errorf("unable to write genId to buffer during b brbsend: %v", err)
	}
	err = binary.Write(writer, binary.LittleEndian, nonce)
	if err != nil {
		return fmt.Errorf("unable to write nonce to buffer during b brbsend: %v", err)
	}
	err = buildMessageContent(writer, msg, brbsend)
	if err != nil {
		return fmt.Errorf("unable to build message content: %v", err)
	}
	b.data.network.Broadcast(buf.Bytes())
	return nil
}

func (b *brbInstance) bebReceive(reader *bytes.Reader, sender *ecdsa.PublicKey) error {
	msg, err := b.deserializeMessage(reader)
	if err != nil {
		return fmt.Errorf("unable to deserialize received message")
	}
	switch msg.kind {
	case brbsend:
		b.handleSend(msg.content)
	case brbecho:
		b.handleEcho(msg.content, msg.id, sender)
	case brbready:
		b.handleReady(msg.content, msg.id, sender)
	default:
		panic("unhandled default case")
	}
	return nil
}

func (b *brbInstance) deserializeMessage(reader *bytes.Reader) (msgStruct, error) {
	typeByte, err := reader.ReadByte()
	if err != nil {
		return msgStruct{}, fmt.Errorf("unable to read type byte from message during beb receive: %v", err)
	}
	msg := make([]byte, reader.Len())
	num, err := reader.Read(msg)
	if err != nil {
		return msgStruct{}, fmt.Errorf("unable to read message contents: %v", err)
	} else if num != len(msg) {
		return msgStruct{}, fmt.Errorf("could not read full message content")
	}
	msgId := crypto.BytesToUUID(msg)
	msgStrct := msgStruct{
		msgId, msg, brb(typeByte),
	}
	return msgStrct, nil
}

func (b *brbInstance) handleSend(msg []byte) {
	b.tasks <- func() error {
		err := b.concreteState.handleSend(msg)
		if err != nil {
			return fmt.Errorf("unable to handle send: %v", err)
		}
		return nil
	}
}

func (b *brbInstance) handleEcho(msg []byte, id UUID, sender *ecdsa.PublicKey) {
	b.tasks <- func() error {
		ok := b.peersEchoed[*sender]
		if ok {
			return fmt.Errorf("already received echo from peer %s", *sender)
		}
		b.data.echoes[id]++
		err := b.concreteState.handleEcho(msg, id)
		if err != nil {
			return fmt.Errorf("unable to handle echo: %v", err)
		}
		return nil
	}
}

func (b *brbInstance) handleReady(msg []byte, id UUID, sender *ecdsa.PublicKey) {
	b.tasks <- func() error {
		ok := b.peersReadied[*sender]
		if ok {
			return fmt.Errorf("already received ready from peer %s", *sender)
		}
		b.data.readies[id]++
		err := b.concreteState.handleReady(msg, id)
		if err != nil {
			return fmt.Errorf("unable to handle ready: %v", err)
		}
		return nil
	}

}

func (b *brbInstance) executorService(tasks <-chan func() error, closeChan <-chan struct{}) {
	for {
		select {
		case task := <-tasks:
			err := task()
			if err != nil {
				slog.Error("unable to compute task", "id", b.data.id, "error", err)
			}
		case <-closeChan:
			slog.Debug("closing brb executor", "id", b.data.id)
			return
		}
	}
}

type brbPhase1Handler struct {
	data       *brbData
	isFinished bool
	nextPhase  brbState
}

type brbPhase2Handler struct {
	data       *brbData
	isFinished bool
	nextPhase  brbState
}

type brbPhase3Handler struct {
	data *brbData
}

func (b *brbPhase1Handler) handleSend(msg []byte) error {
	if b.isFinished {
		return b.nextPhase.handleSend(msg)
	} else {
		return b.sendEcho(msg)
	}
}

func (b *brbPhase1Handler) handleEcho(msg []byte, id UUID) error {
	if b.isFinished {
		return b.nextPhase.handleEcho(msg, id)
	} else {
		numEchoes, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find echoes in phase 1 with message id %s", id)
		}
		if numEchoes == b.data.f+1 {
			err := b.sendEcho(msg)
			if err != nil {
				return fmt.Errorf("unable to send echo in phase 1 upon receiving f+1 echoes: %v", err)
			}
			b.isFinished = true
			return b.nextPhase.handleEcho(msg, id)
		}
	}
	return nil
}

func (b *brbPhase1Handler) handleReady(msg []byte, id UUID) error {
	if b.isFinished {
		return b.nextPhase.handleReady(msg, id)
	} else {
		numReadies, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find readies with message id %s", id)
		}
		if numReadies == b.data.f+1 {
			err := b.sendEcho(msg)
			if err != nil {
				return fmt.Errorf("unable to send echo in phase 1 upon receiving f+1 readies: %v", err)
			}
			b.isFinished = true
			return b.nextPhase.handleReady(msg, id)
		}
	}
	return nil
}

func (b *brbPhase1Handler) sendEcho(msg []byte) error {
	err := sendMessage(msg, brbecho, b.data)
	if err != nil {
		return fmt.Errorf("unable to send echo message: %v", err)
	}
	return nil
}

func (b *brbPhase2Handler) handleSend(msg []byte) error {
	return b.nextPhase.handleSend(msg)
}

func (b *brbPhase2Handler) handleEcho(msg []byte, id UUID) error {
	if b.isFinished {
		return b.nextPhase.handleEcho(msg, id)
	} else {
		numEchoes, ok := b.data.echoes[id]
		if !ok {
			return fmt.Errorf("unable to find echoes in phase 2 with message id %s", id)
		}
		if numEchoes == 2*b.data.f+1 {
			err := b.sendReady(msg)
			if err != nil {
				return fmt.Errorf("unable to send ready in phase 2 upon receiving an echo message: %v", err)
			}
			b.isFinished = true
			return b.nextPhase.handleEcho(msg, id)
		}
	}
	return nil
}

func (b *brbPhase2Handler) handleReady(msg []byte, id UUID) error {
	if b.isFinished {
		return b.nextPhase.handleReady(msg, id)
	} else {
		numReadies, ok := b.data.readies[id]
		if !ok {
			return fmt.Errorf("unable to find readies in phase 2 with message id %s", id)
		}
		if numReadies == b.data.f+1 {
			err := b.sendReady(msg)
			if err != nil {
				return fmt.Errorf("unable to send ready in phase 2 upon receiving a ready message: %v", err)
			}
			b.isFinished = true
			return b.nextPhase.handleReady(msg, id)
		}
	}
	return nil
}

func (b *brbPhase2Handler) sendReady(msg []byte) error {
	err := sendMessage(msg, brbready, b.data)
	if err != nil {
		return fmt.Errorf("unable to send ready message: %v", err)
	}
	return nil
}

func sendMessage(msg []byte, msgType brb, b *brbData) error {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	_, err := writer.Write([]byte{byte(withId)})
	if err != nil {
		return fmt.Errorf("unable to write withId to buffer: %v", err)
	}
	_, err = writer.Write(b.idBytes)
	if err != nil {
		return fmt.Errorf("unable to write broadcast id to buffer: %v", err)
	}
	err = buildMessageContent(writer, msg, msgType)
	if err != nil {
		return fmt.Errorf("unable to write message content: %v", err)
	}
	b.network.Broadcast(buf.Bytes())
	return nil
}

func buildMessageContent(writer *bufio.Writer, msg []byte, msgType brb) error {
	_, err := writer.Write([]byte{byte(brbMsg)})
	if err != nil {
		return fmt.Errorf("unable to write bcbMsg to buffer: %v", err)
	}
	_, err = writer.Write([]byte{byte(msgType)})
	if err != nil {
		return fmt.Errorf("unable to write bcbready to buffer: %v", err)
	}
	num, err := writer.Write(msg)
	if err != nil {
		return fmt.Errorf("unable to write message to buffer: %v", err)
	} else if num != len(msg) {
		return fmt.Errorf("unable to write full message content")
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("unable to flush buffer: %v", err)
	}
	return nil
}

func (b *brbPhase3Handler) handleSend(msg []byte) error {
	return nil
}

func (b *brbPhase3Handler) handleEcho(msg []byte, id UUID) error {
	return nil
}

func (b *brbPhase3Handler) handleReady(msg []byte, id UUID) error {
	numReadies, ok := b.data.readies[id]
	if !ok {
		return fmt.Errorf("unable to find id for ready message in phase 3")
	}
	if numReadies == 2*b.data.f+1 {
		for _, observer := range b.data.observers {
			observer.brbInstanceDeliver(b.data.id, msg)
		}
	}
	return nil
}
