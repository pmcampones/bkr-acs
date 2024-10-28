package raba

import "fmt"

type auxBroadcaster struct {
	auxChan     chan auxMsg
	prevCoin    byte
	initialized bool
	sent        bool
	majs        map[byte]bool
	commands    chan func()
	closeChan   chan struct{}
}

func newAuxBroadcaster(auxChan chan auxMsg) *auxBroadcaster {
	b := &auxBroadcaster{
		auxChan:     auxChan,
		prevCoin:    BOT,
		initialized: false,
		sent:        false,
		majs:        make(map[byte]bool),
		commands:    make(chan func(), 1),
		closeChan:   make(chan struct{}),
	}
	return b
}

func (b *auxBroadcaster) updateMajs(maj byte) {
	b.majs[maj] = true
}

func (b *auxBroadcaster) initialize(prevCoin byte) error {
	if b.initialized {
		return fmt.Errorf("already initialized")
	}
	b.initialized = true
	b.prevCoin = prevCoin
	roundLogger.Info("initialized aux broadcaster", "prevCoin", prevCoin)
	go b.invoker()
	return nil
}

func (b *auxBroadcaster) broadcast(est, aux byte) error {
	roundLogger.Info("scheduling broadcast aux", "est", est, "aux", aux)
	if b.sent {
		return fmt.Errorf("aux already sent")
	}
	broadcastEst := BOT
	if b.canBroadcastEstimate(est) {
		broadcastEst = est
		b.sent = true
	}
	b.commands <- func() {
		roundLogger.Info("broadcasting aux", "aux", aux)
		b.auxChan <- auxMsg{
			est: broadcastEst,
			aux: aux,
		}
	}
	return nil
}

func (b *auxBroadcaster) canBroadcastEstimate(est byte) bool {
	return (est == 1-b.prevCoin && len(b.majs) == 1 && b.majs[est]) || (b.prevCoin == est && !b.majs[1-est])
}

func (b *auxBroadcaster) hasSent() bool {
	return b.sent
}

func (b *auxBroadcaster) invoker() {
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

func (b *auxBroadcaster) close() {
	roundLogger.Info("signaling to close bVal broadcaster")
	if b.initialized {
		b.closeChan <- struct{}{}
	}
}
