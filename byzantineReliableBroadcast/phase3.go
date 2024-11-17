package byzantineReliableBroadcast

import (
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"pace/utils"
	"unsafe"
)

var phase3Logger = utils.GetLogger("BRB Phase 3", slog.LevelDebug)

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
	numReadies, ok := b.data.readies[id]
	if !ok {
		return fmt.Errorf("unable to find id for ready message in phase 3")
	}
	phase3Logger.Debug("processing ready message", "sender", id, "msg", string(msg), "received", numReadies, "required", b.data.n-b.data.f)
	if numReadies == b.data.n-b.data.f {
		sender, content, err := parseMsg(msg)
		if err != nil {
			return fmt.Errorf("unable to parse ready message: %v", err)
		}
		phase3Logger.Info("received enough readies to deliver output message", "issuer", sender, "msg", string(content))
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
