package raba

import "fmt"

type bValBroadcaster struct {
	bValChan    chan bValMsg
	maj         byte
	initialized bool
	sent        []bool
	commands    chan func()
	closeChan   chan struct{}
}

func newBValBroadcaster(bValChan chan bValMsg) *bValBroadcaster {
	b := &bValBroadcaster{
		bValChan:    bValChan,
		maj:         BOT,
		initialized: false,
		sent:        []bool{false, false},
		commands:    make(chan func(), 2), // send 0, send 1
		closeChan:   make(chan struct{}),
	}
	return b
}

func (b *bValBroadcaster) initialize(maj byte) error {
	if b.initialized {
		return fmt.Errorf("already initialized")
	}
	b.initialized = true
	b.maj = maj
	roundLogger.Info("initialized bVal broadcaster", "maj", maj)
	go b.invoker()
	return nil
}

func (b *bValBroadcaster) broadcast(bVal byte) error {
	roundLogger.Info("scheduling broadcast bVal", "bVal", bVal)
	if b.sent[bVal] {
		return fmt.Errorf("bVal %d already sent bVal", bVal)
	}
	b.sent[bVal] = true
	b.commands <- func() {
		roundLogger.Info("broadcasting bVal", "bVal", bVal)
		b.bValChan <- bValMsg{
			bVal: bVal,
			maj:  b.maj,
		}
	}
	return nil
}

func (b *bValBroadcaster) hasSent(bVal byte) bool {
	return b.sent[bVal]
}

func (b *bValBroadcaster) invoker() {
	for {
		select {
		case command := <-b.commands:
			command()
		case <-b.closeChan:
			roundLogger.Info("closing bVal broadcaster")
			return
		}
	}
}

func (b *bValBroadcaster) close() {
	roundLogger.Info("signaling to close bVal broadcaster")
	if b.initialized {
		b.closeChan <- struct{}{}
	}
}
