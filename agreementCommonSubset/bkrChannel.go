package agreementCommonSubset

import (
	aba "bkr-acs/asynchronousBinaryAgreement"
	brb "bkr-acs/byzantineReliableBroadcast"
	"bkr-acs/utils"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"sync"
	"unsafe"
)

var bkrChannelLogger = utils.GetLogger("BKR Channel", slog.LevelWarn)

type bkrProposalMsg struct {
	bkrId    uuid.UUID
	proposal []byte
}

func (m *bkrProposalMsg) marshal() []byte {
	return append(m.bkrId[:], m.proposal...)
}

func (m *bkrProposalMsg) unmarshal(data []byte) error {
	idSize := unsafe.Sizeof(uuid.UUID{})
	if len(data) < int(idSize) {
		return fmt.Errorf("data is too short to unmarshal, expected at least %d bytes, got %d", idSize, len(data))
	}
	m.bkrId = uuid.UUID(data[:idSize])
	m.proposal = data[idSize:]
	return nil
}

type BKRChannel struct {
	f             uint
	abaChannel    *aba.AbaChannel
	brbChannel    *brb.BRBChannel
	participants  []uuid.UUID
	instanceLock  sync.Mutex
	instances     map[uuid.UUID]*bkr
	finished      map[uuid.UUID]bool
	commands      chan func() error
	closeChan     chan struct{}
	closeListener chan struct{}
}

func NewBKRChannel(f uint, abaChannel *aba.AbaChannel, brbChannel *brb.BRBChannel, participants []uuid.UUID) *BKRChannel {
	c := &BKRChannel{
		f:             f,
		abaChannel:    abaChannel,
		brbChannel:    brbChannel,
		participants:  participants,
		instanceLock:  sync.Mutex{},
		instances:     make(map[uuid.UUID]*bkr),
		finished:      make(map[uuid.UUID]bool),
		commands:      make(chan func() error),
		closeChan:     make(chan struct{}, 1),
		closeListener: make(chan struct{}, 1),
	}
	bkrChannelLogger.Info("initializing channel", "f", f, "participants", participants)
	go c.listenBroadcasts()
	go c.invoker()
	return c
}

func (c *BKRChannel) Propose(id uuid.UUID, proposal []byte) (chan [][]byte, error) {
	msg := &bkrProposalMsg{bkrId: id, proposal: proposal}
	data := msg.marshal()
	bkrChannelLogger.Debug("broadcasting proposal", "id", id, "proposal", string(proposal))
	if err := c.brbChannel.BRBroadcast(data); err != nil {
		return nil, fmt.Errorf("unable to broadcast message: %w", err)
	}
	return c.getInstance(id).output, nil
}

func (c *BKRChannel) listenBroadcasts() {
	for {
		select {
		case msg := <-c.brbChannel.BrbDeliver:
			if err := c.processBroadcast(msg); err != nil {
				bkrChannelLogger.Warn("unable to process broadcast message", "sender", msg.Sender, "error", err)
			}
		case <-c.closeListener:
			bkrChannelLogger.Info("closing listener")
			return
		}
	}
}

func (c *BKRChannel) processBroadcast(msg brb.BRBMsg) error {
	bkrMsg := &bkrProposalMsg{}
	if err := bkrMsg.unmarshal(msg.Content); err != nil {
		return fmt.Errorf("unable to unmarshal message: %w", err)
	}
	go func() {
		c.commands <- func() error {
			if err := c.submitProposal(bkrMsg.bkrId, bkrMsg.proposal, msg.Sender); err != nil {
				return fmt.Errorf("unable to submit proposal: %w", err)
			}
			return nil
		}
	}()
	return nil
}

func (c *BKRChannel) submitProposal(bkrId uuid.UUID, proposal []byte, sender uuid.UUID) error {
	bkrChannelLogger.Debug("submitting proposal", "id", bkrId, "proposal", string(proposal), "sender", sender)
	if c.finished[bkrId] {
		return fmt.Errorf("bkr instance %s is already finished", bkrId)
	} else if err := c.getInstance(bkrId).receiveInput(proposal, sender); err != nil {
		return fmt.Errorf("unable to submit proposal to bkrInstance: %w", err)
	}
	return nil
}

func (c *BKRChannel) getInstance(bkrId uuid.UUID) *bkr {
	bkrInstance := c.instances[bkrId]
	if bkrInstance == nil {
		bkrInstance = c.createNewInstance(bkrId)
	}
	return bkrInstance
}

func (c *BKRChannel) createNewInstance(bkrId uuid.UUID) *bkr {
	bkrChannelLogger.Debug("creating new bkr instance", "id", bkrId)
	c.instanceLock.Lock()
	defer c.instanceLock.Unlock()
	bkrInstance := c.instances[bkrId]
	if bkrInstance != nil {
		return bkrInstance
	}
	bkrInstance = newBKR(bkrId, c.f, c.participants, c.abaChannel)
	c.instances[bkrId] = bkrInstance
	return bkrInstance
}

func (c *BKRChannel) invoker() {
	for {
		select {
		case cmd := <-c.commands:
			if err := cmd(); err != nil {
				bkrChannelLogger.Warn("unable to execute command", "error", err)
			}
		case <-c.closeChan:
			bkrChannelLogger.Info("closing invoker")
			return
		}
	}
}

func (c *BKRChannel) Close() {
	bkrChannelLogger.Info("sending signal to close invoker")
	c.closeChan <- struct{}{}
}
