package overlayNetwork

import "crypto/ecdsa"

type BEBMsg struct {
	Content []byte
	Sender  *ecdsa.PublicKey
}

type BEBChannel struct {
	node        *Node
	listenCode  byte
	deliverChan chan BEBMsg
}

func CreateBEBChannel(node *Node, listenCode byte) *BEBChannel {
	beb := &BEBChannel{
		node:        node,
		listenCode:  listenCode,
		deliverChan: make(chan BEBMsg),
	}
	node.AttachMessageObserver(beb)
	return beb
}

func (b *BEBChannel) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	if msg[0] == b.listenCode {
		b.deliverChan <- BEBMsg{Content: msg[1:], Sender: sender}
	}
}

func (b *BEBChannel) BEBBroadcast(msg []byte) error {
	return b.node.Broadcast(append([]byte{b.listenCode}, msg...))
}
