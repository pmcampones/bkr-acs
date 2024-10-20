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
	node.attachMessageObserver(beb)
	return beb
}

func (b *BEBChannel) BEBroadcast(msg []byte) error {
	wrappedMsg := append([]byte{b.listenCode}, msg...)
	peers := b.node.getPeers()
	if err := b.node.unicastSelf(wrappedMsg); err != nil {
		return err
	}
	for _, peer := range peers {
		err := b.node.unicast(wrappedMsg, peer.conn)
		if err != nil {
			logger.Warn("error sending to connection", "peer name", peer.name, "error", err)
		}
	}
	return nil
}

func (b *BEBChannel) bebDeliver(msg []byte, sender *ecdsa.PublicKey) {
	if msg[0] == b.listenCode {
		b.deliverChan <- BEBMsg{Content: msg[1:], Sender: sender}
	}
}

func (b *BEBChannel) GetBEBChan() <-chan BEBMsg {
	return b.deliverChan
}
