package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"pace/utils"
	"sync"
)

type TestMsgObserver struct {
	delivered map[string]bool
	barrier   chan struct{}
	lock      sync.Mutex
}

func (to *TestMsgObserver) BEBDeliver(msg []byte, _ *ecdsa.PublicKey) {
	to.lock.Lock()
	defer to.lock.Unlock()
	to.delivered[string(msg)] = true
	to.barrier <- struct{}{}
}

type TestMemObserver struct {
	Peers       map[string]*Peer
	UpBarrier   chan struct{}
	DownBarrier chan struct{}
	lock        sync.Mutex
}

func (to *TestMemObserver) NotifyPeerUp(p *Peer) {
	to.lock.Lock()
	defer to.lock.Unlock()
	to.Peers[p.name] = p
	to.UpBarrier <- struct{}{}
}

func (to *TestMemObserver) NotifyPeerDown(_ *Peer) {
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
		Peers:     make(map[string]*Peer),
		UpBarrier: make(chan struct{}, bufferMsg),
	}
	node.AttachMembershipObserver(&memObs)
	msgObs := TestMsgObserver{
		delivered: make(map[string]bool),
		barrier:   make(chan struct{}, bufferMem),
	}
	node.AttachMessageObserver(&msgObs)
	err = node.Join(contact)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to join the overlayNetwork: %v", err)
	}
	return node, &memObs, &msgObs, nil
}
