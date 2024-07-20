package network

import (
	"broadcast_channels/crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/magiconair/properties/assert"
	"sync"
	"testing"
)

type testObserver struct {
	delivered map[string]bool
	barrier   chan<- struct{}
	lock      sync.RWMutex
}

func (to *testObserver) BEBDeliver(msg []byte, _ *ecdsa.PublicKey) {
	to.lock.Lock()
	defer to.lock.Unlock()
	to.delivered[string(msg)] = true
	to.barrier <- struct{}{}
}

func TestShouldBroadcastSingleNode(t *testing.T) {
	contact := "localhost:6000"
	node, err := makeNode(contact, contact)
	if err != nil {
		t.Fatalf("unable to create node sender: %v", err)
	}
	barrier := make(chan struct{})
	obs := testObserver{
		delivered: make(map[string]bool),
		barrier:   barrier,
	}
	node.AddObserver(&obs)
	msg := []byte("hello")
	assert.Equal(t, len(obs.delivered), 0)
	node.Broadcast(msg)
	<-barrier
	assert.Equal(t, obs.delivered[string(msg)], true)
	assert.Equal(t, len(obs.delivered), 1)
}

func TestShouldBroadcastSingleNodeManyMessages(t *testing.T) {
	contact := "localhost:6000"
	numMsgs := 10000
	msgs := make([][]byte, numMsgs)
	for i := 0; i < numMsgs; i++ {
		msgs[i] = []byte(fmt.Sprintf("hello %d", i))
	}
	node, err := makeNode(contact, contact)
	if err != nil {
		t.Fatalf("unable to create node sender: %v", err)
	}
	barrier := make(chan struct{}, numMsgs)
	obs := testObserver{
		delivered: make(map[string]bool),
		barrier:   barrier,
	}
	node.AddObserver(&obs)
	assert.Equal(t, len(obs.delivered), 0)
	for _, msg := range msgs {
		node.Broadcast(msg)
	}
	for i := 0; i < numMsgs; i++ {
		<-barrier
	}
	for _, msg := range msgs {
		assert.Equal(t, obs.delivered[string(msg)], true)
	}
	assert.Equal(t, len(obs.delivered), len(msgs))
}

func makeNode(address, contact string) (*Node, error) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("unable to generate key: %v", err)
	}
	cert, err := crypto.MakeSelfSignedCert(sk)
	if err != nil {
		return nil, fmt.Errorf("unable to create cert: %v", err)
	}
	node, err := Join(address, contact, sk, cert)
	if err != nil {
		return nil, fmt.Errorf("unable to join the network: %v", err)
	}
	return node, nil
}
