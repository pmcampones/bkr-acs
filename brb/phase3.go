package brb

import (
	"fmt"
	"github.com/google/uuid"
	"unsafe"
)

var idLen = unsafe.Sizeof(uuid.UUID{})

type brbPhase3Handler struct {
	data       *brbData
	outputChan chan<- BRBMsg
}

func newPhase3Handler(data *brbData, output chan<- BRBMsg) *brbPhase3Handler {
	return &brbPhase3Handler{
		data:       data,
		outputChan: output,
	}
}

func (b *brbPhase3Handler) handleReady(msg []byte, id uuid.UUID) error {
	instanceLogger.Debug("processing ready message on phase 3")
	numReadies, ok := b.data.readies[id]
	if !ok {
		return fmt.Errorf("unable to find id for ready message in phase 3")
	}
	if numReadies == b.data.n-b.data.f {
		instanceLogger.Info("sending output message")
		sender, content, err := parseMsg(msg)
		if err != nil {
			return fmt.Errorf("unable to parse ready message: %v", err)
		}
		b.outputChan <- BRBMsg{
			Content: content,
			Sender:  sender,
		}
	}
	return nil
}

func parseMsg(msg []byte) (uuid.UUID, []byte, error) {
	idBytes := msg[:idLen]
	if sender, err := uuid.FromBytes(idBytes); err != nil {
		return uuid.UUID{}, nil, fmt.Errorf("unable to parse sender id: %v", err)
	} else {
		return sender, msg[idLen:], nil
	}
}
