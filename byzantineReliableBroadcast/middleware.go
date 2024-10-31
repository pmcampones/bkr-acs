package byzantineReliableBroadcast

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	on "pace/overlayNetwork"
	"pace/utils"
)

type middlewareCode byte

const (
	send middlewareCode = 'a' + iota
	echo
	ready
)

type msg struct {
	kind    middlewareCode
	id      uuid.UUID
	sender  uuid.UUID
	content []byte
}

type brbMiddleware struct {
	bebChannel  *on.BEBChannel
	deliverChan chan<- *msg
	closeChan   chan struct{}
}

func newBRBMiddleware(bebChannel *on.BEBChannel, deliverChan chan<- *msg) *brbMiddleware {
	m := &brbMiddleware{
		bebChannel:  bebChannel,
		deliverChan: deliverChan,
		closeChan:   make(chan struct{}),
	}
	go m.bebDeliver(bebChannel.GetBEBChan())
	return m
}

func (m *brbMiddleware) bebDeliver(bebChan <-chan on.BEBMsg) {
	for {
		select {
		case bebMsg := <-bebChan:
			structMsg, err := m.processMsg(bebMsg.Content, bebMsg.Sender)
			if err != nil {
				channelLogger.Warn("unable to processMsg message during beb delivery", "error", err)
			} else {
				channelLogger.Debug("received message from beb", "sender", structMsg.sender, "type", structMsg.kind, "msg", string(structMsg.content))
				go func() { m.deliverChan <- structMsg }()
			}
		case <-m.closeChan:
			channelLogger.Info("closing byzantineReliableBroadcast middleware")
			return
		}
	}
}

func (m *brbMiddleware) makeChannels(id uuid.UUID) (chan []byte, chan []byte) {
	echoChan := make(chan []byte)
	go m.broadcastMsgOnSignal(echo, id, echoChan)
	readyChan := make(chan []byte)
	go m.broadcastMsgOnSignal(ready, id, readyChan)
	return echoChan, readyChan
}

func (m *brbMiddleware) broadcastSend(msg []byte) error {
	structuredMsg, err := m.wrapSend(msg)
	if err != nil {
		return fmt.Errorf("error wrapping send: %v", err)
	} else if err = m.bebChannel.BEBroadcast(structuredMsg); err != nil {
		return fmt.Errorf("error broadcasting send: %v", err)
	}
	return nil
}

func (m *brbMiddleware) wrapSend(msg []byte) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	nonce := rand.Uint32()
	if _, err := writer.Write([]byte{byte(send)}); err != nil {
		return nil, fmt.Errorf("unable to write send code to buffer: %v", err)
	} else if err := binary.Write(writer, binary.LittleEndian, nonce); err != nil {
		return nil, fmt.Errorf("unable to write nonce to buffer: %v", err)
	} else if _, err := writer.Write(msg); err != nil {
		return nil, fmt.Errorf("unable to write message to buffer: %v", err)
	} else if err := writer.Flush(); err != nil {
		return nil, fmt.Errorf("unable to flush writer: %v", err)
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
	if err := binary.Write(buf, binary.LittleEndian, nonce); err != nil {
		return uuid.Nil, fmt.Errorf("unable to write nonce to buffer during byzantineReliableBroadcast: %v", err)
	}
	id := utils.BytesToUUID(buf.Bytes())
	return id, nil
}

func (m *brbMiddleware) broadcastMsgOnSignal(code middlewareCode, id uuid.UUID, ch chan []byte) {
	msg := <-ch
	m.broadcastMsg(code, id, msg)
}

func (m *brbMiddleware) broadcastMsg(code middlewareCode, id uuid.UUID, msg []byte) {
	structuredMsg, err := m.wrapMessage(code, id, msg)
	if err != nil {
		channelLogger.Warn("error wrapping message", "error", err)
		return
	} else if err = m.bebChannel.BEBroadcast(structuredMsg); err != nil {
		channelLogger.Warn("error broadcasting message", "error", err)
	}
}

func (m *brbMiddleware) wrapMessage(code middlewareCode, id uuid.UUID, msg []byte) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	if _, err := writer.Write([]byte{byte(code)}); err != nil {
		return nil, fmt.Errorf("unable to write codes to buffer: %v", err)
	}
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal id: %v", err)
	} else if _, err := writer.Write(idBytes); err != nil {
		return nil, fmt.Errorf("unable to write id to buffer: %v", err)
	} else if _, err := writer.Write(msg); err != nil {
		return nil, fmt.Errorf("unable to write message to buffer: %v", err)
	} else if err := writer.Flush(); err != nil {
		return nil, fmt.Errorf("unable to flush writer: %v", err)
	}
	return buf.Bytes(), nil
}

func (m *brbMiddleware) processMsg(msg []byte, sender *ecdsa.PublicKey) (*msg, error) {
	reader := bytes.NewReader(msg)
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
		return nil, fmt.Errorf("unable to processMsg id generation: %v", err)
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
		return nil, fmt.Errorf("unable to convert sender public key to UUID: %w", err)
	}
	content := make([]byte, reader.Len())
	if n, err := reader.Read(content); err != nil || n != len(content) {
		return nil, fmt.Errorf("unable to read content: %v", err)
	}
	return &msg{
		kind:    kind,
		id:      id,
		sender:  senderId,
		content: content,
	}, nil
}
