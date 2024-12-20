package asynchronousBinaryAgreement

import (
	on "bkr-acs/overlayNetwork"
	"bkr-acs/utils"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
)

var termLogger = utils.GetLogger("ABA Termination Middleware", slog.LevelWarn)

type terminationMsg struct {
	sender   uuid.UUID
	instance uuid.UUID
	decision byte
}

type terminationMiddleware struct {
	beb       *on.BEBChannel
	output    chan *terminationMsg
	closeChan chan struct{}
}

func newTerminationMiddleware(beb *on.BEBChannel) *terminationMiddleware {
	tg := &terminationMiddleware{
		beb:       beb,
		output:    make(chan *terminationMsg),
		closeChan: make(chan struct{}),
	}
	go tg.brbDeliver()
	termLogger.Info("new termination middleware created")
	return tg
}

func (m *terminationMiddleware) brbDeliver() {
	for {
		select {
		case brbMsg := <-m.beb.GetBEBChan():
			if err := m.processMsg(brbMsg); err != nil {
				termLogger.Warn("unable to process termination message", "error", err)
			}
		case <-m.closeChan:
			termLogger.Info("closing termination gadget")
			return
		}
	}
}

func (m *terminationMiddleware) processMsg(brbMsg on.BEBMsg) error {
	senderId, err := utils.PkToUUID(brbMsg.Sender)
	if err != nil {
		return fmt.Errorf("unable to convert public key to uuid: %v", err)
	}
	if tm, err := m.parseMsg(brbMsg.Content, senderId); err != nil {
		termLogger.Warn("unable to parse termination message", "error", err)
	} else {
		termLogger.Debug("received termination message", "sender", tm.sender, "inner", tm.instance, "decision", tm.decision)
		go func() { m.output <- tm }()
	}
	return nil
}

func (m *terminationMiddleware) parseMsg(msg []byte, sender uuid.UUID) (*terminationMsg, error) {
	tm := &terminationMsg{}
	tm.sender = sender
	reader := bytes.NewReader(msg)
	if id, err := utils.ExtractIdFromMessage(reader); err != nil {
		return nil, fmt.Errorf("unable to extract inner id from termination message: %v", err)
	} else {
		tm.instance = id
	}
	if err := binary.Read(reader, binary.LittleEndian, &tm.decision); err != nil {
		return nil, fmt.Errorf("unable to read decision from termination message: %v", err)
	}
	return tm, nil
}

func (m *terminationMiddleware) broadcastDecision(instance uuid.UUID, decision byte) error {
	termLogger.Debug("broadcasting termination decision", "instance", instance, "decision", decision)
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	if idBytes, err := instance.MarshalBinary(); err != nil {
		return fmt.Errorf("unable to marshal inner id: %v", err)
	} else if _, err := writer.Write(idBytes); err != nil {
		return fmt.Errorf("unable to write inner id to termination message: %v", err)
	} else if err := binary.Write(writer, binary.LittleEndian, decision); err != nil {
		return fmt.Errorf("unable to write decision to termination message: %v", err)
	} else if err := writer.Flush(); err != nil {
		return fmt.Errorf("unable to flush termination message buffer: %v", err)
	} else if err := m.beb.BEBroadcast(buf.Bytes()); err != nil {
		return fmt.Errorf("unable to broadcast termination message: %v", err)
	}
	return nil
}

func (m *terminationMiddleware) close() {
	termLogger.Info("signaling close termination middleware")
	m.closeChan <- struct{}{}
}
