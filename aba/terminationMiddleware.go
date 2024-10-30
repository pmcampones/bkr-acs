package aba

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/brb"
	"pace/utils"
)

var termLogger = utils.GetLogger(slog.LevelDebug)

type terminationMsg struct {
	sender   uuid.UUID
	instance uuid.UUID
	round    uint16
	decision byte
}

type terminationMiddleware struct {
	brb       *brb.BRBChannel
	output    chan *terminationMsg
	closeChan chan struct{}
}

func newTerminationMiddleware(brb *brb.BRBChannel) *terminationMiddleware {
	tg := &terminationMiddleware{
		brb:       brb,
		output:    make(chan *terminationMsg),
		closeChan: make(chan struct{}),
	}
	go tg.listenDecisions()
	return tg
}

func (m *terminationMiddleware) listenDecisions() {
	for {
		select {
		case brbMsg := <-m.brb.BrbDeliver:
			tm, err := m.parseMsg(brbMsg.Content, brbMsg.Sender)
			if err != nil {
				termLogger.Warn("unable to parse termination message", "error", err)
			} else {
				termLogger.Debug("received termination message", "sender", tm.sender, "instance", tm.instance, "round", tm.round, "decision", tm.decision)
				go func() { m.output <- tm }()
			}
		case <-m.closeChan:
			termLogger.Info("closing termination gadget")
			return
		}
	}
}

func (m *terminationMiddleware) parseMsg(msg []byte, sender uuid.UUID) (*terminationMsg, error) {
	tm := &terminationMsg{}
	tm.sender = sender
	reader := bytes.NewReader(msg)
	if id, err := utils.ExtractIdFromMessage(reader); err != nil {
		return nil, fmt.Errorf("unable to extract instance id from termination message: %v", err)
	} else {
		tm.instance = id
	}
	if err := binary.Read(reader, binary.LittleEndian, &tm.round); err != nil {
		return nil, fmt.Errorf("unable to read round from termination message: %v", err)
	} else if err := binary.Read(reader, binary.LittleEndian, &tm.decision); err != nil {
		return nil, fmt.Errorf("unable to read decision from termination message: %v", err)
	}
	return tm, nil
}

func (m *terminationMiddleware) broadcastDecision(instance uuid.UUID, round uint16, decision byte) error {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	if idBytes, err := instance.MarshalBinary(); err != nil {
		return fmt.Errorf("unable to marshal instance id: %v", err)
	} else if _, err := writer.Write(idBytes); err != nil {
		return fmt.Errorf("unable to write instance id to termination message: %v", err)
	} else if err := binary.Write(writer, binary.LittleEndian, round); err != nil {
		return fmt.Errorf("unable to write round to termination message: %v", err)
	} else if err := binary.Write(writer, binary.LittleEndian, decision); err != nil {
		return fmt.Errorf("unable to write decision to termination message: %v", err)
	} else if err := writer.Flush(); err != nil {
		return fmt.Errorf("unable to flush termination message buffer: %v", err)
	} else if err := m.brb.BRBroadcast(buf.Bytes()); err != nil {
		return fmt.Errorf("unable to broadcast termination message: %v", err)
	}
	return nil
}

func (m *terminationMiddleware) close() {
	termLogger.Info("signaling close termination middleware")
	m.closeChan <- struct{}{}
}
