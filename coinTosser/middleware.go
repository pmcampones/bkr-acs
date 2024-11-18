package coinTosser

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/overlayNetwork"
	"pace/utils"
)

var middlewareLogger = utils.GetLogger("CT Middleware", slog.LevelWarn)

type msg struct {
	id     uuid.UUID
	sender uuid.UUID
	share  ctShare
}

type ctMiddleware struct {
	bebChannel  *overlayNetwork.BEBChannel
	deliverChan chan<- *msg
	closeChan   chan struct{}
}

func newCTMiddleware(bebChannel *overlayNetwork.BEBChannel, deliverChan chan<- *msg) *ctMiddleware {
	m := &ctMiddleware{
		bebChannel:  bebChannel,
		deliverChan: deliverChan,
		closeChan:   make(chan struct{}, 1),
	}
	go m.bebDeliver(bebChannel.GetBEBChan())
	middlewareLogger.Info("new CT middleware created")
	return m
}

func (m *ctMiddleware) bebDeliver(bebChan <-chan overlayNetwork.BEBMsg) {
	for {
		select {
		case bebMsg := <-bebChan:
			structMsg, err := m.processMsg(bebMsg.Content, bebMsg.Sender)
			if err != nil {
				channelLogger.Warn("unable to processMsg message during beb delivery", "error", err)
			}
			middlewareLogger.Debug("beb message delivered", "instance", structMsg.id, "sender", structMsg.sender, "share", structMsg.share)
			m.deliverChan <- structMsg
		case <-m.closeChan:
			return
		}
	}
}

func (m *ctMiddleware) processMsg(content []byte, sender *ecdsa.PublicKey) (*msg, error) {
	reader := bytes.NewReader(content)
	id, err := utils.ExtractIdFromMessage(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to extract id from message: %v", err)
	}
	senderId, err := utils.PkToUUID(sender)
	if err != nil {
		return nil, fmt.Errorf("unable to convert sender public key to UUID: %v", err)
	}
	share := emptyCTShare()
	shareBytes := make([]byte, reader.Len())
	if _, err := reader.Read(shareBytes); err != nil {
		return nil, fmt.Errorf("unable to read share bytes: %v", err)
	} else if err := share.unmarshalBinary(shareBytes); err != nil {
		return nil, fmt.Errorf("unable to unmarshal share: %v", err)
	}
	return &msg{
		id:     id,
		sender: senderId,
		share:  share,
	}, nil
}

func (m *ctMiddleware) broadcastCTShare(id uuid.UUID, share ctShare) error {
	middlewareLogger.Debug("broadcasting CT share", "instance", id, "share", share)
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return fmt.Errorf("unable to marshal id: %v", err)
	}
	shareBytes, err := share.marshalBinary()
	if err != nil {
		return fmt.Errorf("unable to marshal share: %v", err)
	}
	msgBytes := append(idBytes, shareBytes...)
	return m.bebChannel.BEBroadcast(msgBytes)
}
