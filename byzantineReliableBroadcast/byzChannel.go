package byzantineReliableBroadcast

import (
	on "bkr-acs/overlayNetwork"
	"github.com/google/uuid"
)

const byzMsg = "I am byzantine and I hate candy :("

type byzChannel struct {
	middleware *brbMiddleware
	received   map[uuid.UUID]bool
	closeChan  chan struct{}
}

func createByzChannel(beb *on.BEBChannel) *byzChannel {
	deliverChan := make(chan *msg)
	byz := &byzChannel{
		middleware: newBRBMiddleware(beb, deliverChan),
		received:   make(map[uuid.UUID]bool),
		closeChan:  make(chan struct{}, 1),
	}
	go byz.bebDeliver(deliverChan)
	return byz
}

func (b *byzChannel) bebDeliver(deliverChan <-chan *msg) {
	for {
		select {
		case msg := <-deliverChan:
			b.processMsg(msg)
		case <-b.closeChan:
			return
		}
	}
}

func (b *byzChannel) processMsg(msg *msg) {
	if !b.received[msg.id] {
		b.received[msg.id] = true
		b.middleware.broadcastMsg(ready, msg.id, []byte(byzMsg))
		b.middleware.broadcastMsg(ready, msg.id, []byte(byzMsg))
		b.middleware.broadcastMsg(echo, msg.id, []byte(byzMsg))
		b.middleware.broadcastMsg(echo, msg.id, []byte(byzMsg))
	}
}

func (b *byzChannel) close() {
	b.closeChan <- struct{}{}
}
