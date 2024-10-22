package coinTosser

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"github.com/google/uuid"
	"pace/overlayNetwork"
	"pace/utils"
)

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
		closeChan:   make(chan struct{}),
	}
	go m.bebDeliver(bebChannel.GetBEBChan())
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
