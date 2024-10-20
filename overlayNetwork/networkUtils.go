package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"pace/utils"
	"sync"
	"testing"
)

type TestMsgObserver struct {
	delivered map[string]bool
	barrier   chan struct{}
	lock      sync.Mutex
}

func (to *TestMsgObserver) bebDeliver(msg []byte, _ *ecdsa.PublicKey) {
	to.lock.Lock()
	defer to.lock.Unlock()
	to.delivered[string(msg)] = true
	to.barrier <- struct{}{}
}

type TestMemObserver struct {
	Peers       map[string]*peer
	UpBarrier   chan struct{}
	DownBarrier chan struct{}
	lock        sync.Mutex
}

func (to *TestMemObserver) NotifyPeerUp(p *peer) {
	to.lock.Lock()
	defer to.lock.Unlock()
	to.Peers[p.name] = p
	to.UpBarrier <- struct{}{}
}

func (to *TestMemObserver) NotifyPeerDown(_ *peer) {
	/*to.lock.Lock()
	defer to.lock.Unlock()
	to.Peers[p.name] = p
	go func() { to.DownBarrier <- struct{}{} }()*/
}

func MakeNode(address, contact string, bufferMsg, bufferMem int) (*Node, *TestMemObserver, *TestMsgObserver, error) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to generate key: %v", err)
	}
	cert, err := utils.MakeSelfSignedCert(sk)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to create cert: %v", err)
	}
	node := NewNode(address, contact, sk, cert)
	memObs := TestMemObserver{
		Peers:     make(map[string]*peer),
		UpBarrier: make(chan struct{}, bufferMsg),
	}
	msgObs := TestMsgObserver{
		delivered: make(map[string]bool),
		barrier:   make(chan struct{}, bufferMem),
	}
	node.attachMessageObserver(&msgObs)
	err = node.Join()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to join the overlayNetwork: %v", err)
	}
	return node, &memObs, &msgObs, nil
}

func GetNode(t *testing.T, address, contact string) *Node {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)
	cert, err := utils.MakeSelfSignedCert(sk)
	assert.NoError(t, err)
	return NewNode(address, contact, sk, cert)
}

func InitializeNodes(t *testing.T, nodes []*Node) {
	memChans := lo.Map(nodes, func(n *Node, _ int) chan struct{} { return n.memChan })
	for _, n := range nodes {
		err := n.Join()
		assert.NoError(t, err)
	}
	for _, ch := range memChans {
		for i := 0; i < len(nodes)-1; i++ {
			<-ch
		}
	}
}
