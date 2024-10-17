package byzantineReliableBroadcast

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"pace/overlayNetwork"
	"pace/utils"
)

type middlewareCode byte

const (
	send middlewareCode = 'a' + iota
	echo
	ready
)

type brbMiddleware struct {
	bebChannel *overlayNetwork.Node
	listenCode byte
}

func newBRBMiddleware(bebChannel *overlayNetwork.Node, code byte) *brbMiddleware {
	return &brbMiddleware{
		bebChannel: bebChannel,
		listenCode: code,
	}
}

func (m *brbMiddleware) makeChannels(id uuid.UUID) (chan []byte, chan []byte) {
	echoChan := make(chan []byte)
	go m.broadcastMsg(echo, id, echoChan)
	readyChan := make(chan []byte)
	go m.broadcastMsg(ready, id, readyChan)
	return echoChan, readyChan
}

func (m *brbMiddleware) broadcastSend(msg []byte) error {
	structuredMsg, err := m.wrapSend(msg)
	if err != nil {
		return fmt.Errorf("error wrapping send: %v", err)
	} else if err = m.bebChannel.Broadcast(structuredMsg); err != nil {
		return fmt.Errorf("error broadcasting send: %v", err)
	} else {
		return nil
	}
}

func (m *brbMiddleware) wrapSend(msg []byte) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	_, err := writer.Write([]byte{m.listenCode, byte(send)})
	if err != nil {
		return nil, fmt.Errorf("unable to write codes to buffer: %v", err)
	}
	nonce := rand.Uint32()
	err = binary.Write(writer, binary.LittleEndian, nonce)
	if err != nil {
		return nil, fmt.Errorf("unable to write nonce to buffer: %v", err)
	}
	_, err = writer.Write(msg)
	if err != nil {
		return nil, fmt.Errorf("unable to write message to buffer: %v", err)
	}
	return buf.Bytes(), nil
}

func (m *brbMiddleware) genId(sender *ecdsa.PublicKey) (uuid.UUID, error) {
	nonce := rand.Uint32()
	encodedPk, err := utils.SerializePublicKey(sender)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to serialize public key during byzantineReliableBroadcast: %v", err)
	}
	buf := bytes.NewBuffer(encodedPk)
	err = binary.Write(buf, binary.LittleEndian, nonce)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to write nonce to buffer during byzantineReliableBroadcast: %v", err)
	}
	id := utils.BytesToUUID(buf.Bytes())
	return id, nil
}

func (m *brbMiddleware) broadcastMsg(code middlewareCode, id uuid.UUID, ch chan []byte) {
	msg := <-ch
	structuredMsg, err := m.wrapMessage(code, id, msg)
	if err != nil {
		logger.Warn("error wrapping message", "error", err)
		return
	} else if err = m.bebChannel.Broadcast(structuredMsg); err != nil {
		logger.Warn("error broadcasting message", "error", err)
	}
}

func (m *brbMiddleware) wrapMessage(code middlewareCode, id uuid.UUID, msg []byte) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	_, err := writer.Write([]byte{m.listenCode, byte(code)})
	if err != nil {
		return nil, fmt.Errorf("unable to write codes to buffer: %v", err)
	}
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal id: %v", err)
	}
	_, err = writer.Write(idBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to write id to buffer: %v", err)
	}
	_, err = writer.Write(msg)
	if err != nil {
		return nil, fmt.Errorf("unable to write message to buffer: %v", err)
	}
	return buf.Bytes(), nil
}

type msg struct {
	kind    middlewareCode
	id      uuid.UUID
	sender  uuid.UUID
	content []byte
}

func (m *brbMiddleware) processMsg(msg []byte, sender *ecdsa.PublicKey) (*msg, error) {
	reader := bytes.NewReader(msg[1:])
	byteType, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("unable to read byte type from message during instance id computation: %v", err)
	}
	kind := middlewareCode(byteType)
	if kind == send {
		return m.processSend(reader, sender)
	} else if kind == echo || kind == ready {
		return m.deserializeMsg(kind, reader, sender)
	} else {
		return nil, fmt.Errorf("unhandled default case in instance id computation")
	}
}

func (m *brbMiddleware) processSend(reader *bytes.Reader, sender *ecdsa.PublicKey) (*msg, error) {
	id, err := m.readId(reader, sender)
	if err != nil {
		return nil, fmt.Errorf("unable to process id generation: %v", err)
	}
	return m.deserializeIDMsg(send, reader, sender, id)
}

func (m *brbMiddleware) readId(reader *bytes.Reader, sender *ecdsa.PublicKey) (uuid.UUID, error) {
	var nonce uint32
	err := binary.Read(reader, binary.LittleEndian, &nonce)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to read nonce from message during instance id computation: %v", err)
	}
	return m.computeInstanceId(nonce, sender)
}

func (m *brbMiddleware) computeInstanceId(nonce uint32, sender *ecdsa.PublicKey) (uuid.UUID, error) {
	encodedPk, err := utils.SerializePublicKey(sender)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("bcb channel unable to serialize public key: %v", err)
	}
	buf := bytes.NewBuffer(encodedPk)
	err = binary.Write(buf, binary.LittleEndian, nonce)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("unable to write nonce to buffer: %v", err)
	}
	id := utils.BytesToUUID(buf.Bytes())
	return id, nil
}

func (m *brbMiddleware) deserializeMsg(kind middlewareCode, reader *bytes.Reader, sender *ecdsa.PublicKey) (*msg, error) {
	id, err := utils.ExtractIdFromMessage(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to extract id from message: %v", err)
	}
	return m.deserializeIDMsg(kind, reader, sender, id)
}

func (m *brbMiddleware) deserializeIDMsg(kind middlewareCode, reader *bytes.Reader, sender *ecdsa.PublicKey, id uuid.UUID) (*msg, error) {
	senderId, err := utils.PkToUUID(sender)
	if err != nil {
		return nil, fmt.Errorf("unable to convert sender to UUID: %v", err)
	}
	content := make([]byte, reader.Len())
	_, err = reader.Read(content)
	if err != nil {
		return nil, fmt.Errorf("unable to read content: %v", err)
	}
	return &msg{
		kind:    kind,
		id:      id,
		sender:  senderId,
		content: content,
	}, nil
}
