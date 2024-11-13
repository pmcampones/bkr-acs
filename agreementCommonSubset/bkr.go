package agreementCommonSubset

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"log/slog"
	aba "pace/asynchronousBinaryAgreement"
	"pace/utils"
	"slices"
)

var bkrLogger = utils.GetLogger(slog.LevelDebug)

const (
	accept = 1
	reject = 0
	bot    = 2
)

type BKR struct {
	id           uuid.UUID
	f            uint
	participants []uuid.UUID
	iProposed    []bool
	inputs       [][]byte
	results      []byte
	abaInstances []*aba.AbaInstance
	hasDelivered bool
	output       chan [][]byte
	commands     chan func() error
	closeChan    chan struct{}
}

func newBKR(id uuid.UUID, f uint, participants []uuid.UUID, abaChan *aba.AbaChannel) (*BKR, error) {
	bkrLogger.Info("initializing BKR", "id", id, "f", f, "participants", participants)
	bkr := &BKR{
		id:           id,
		f:            f,
		participants: participants,
		iProposed:    make([]bool, len(participants)),
		inputs:       make([][]byte, len(participants)),
		results:      lo.Map(participants, func(_ uuid.UUID, _ int) byte { return bot }),
		hasDelivered: false,
		output:       make(chan [][]byte, 1),
		commands:     make(chan func() error),
		closeChan:    make(chan struct{}, 1),
	}
	abaIds, err := bkr.computeAbaIds(participants)
	if err != nil {
		return nil, fmt.Errorf("unable to compute aba ids: %w", err)
	}
	bkr.abaInstances = lo.Map(abaIds, func(abaId uuid.UUID, _ int) *aba.AbaInstance { return abaChan.NewAbaInstance(abaId) })
	for i, instance := range bkr.abaInstances {
		go bkr.handleAbaResponse(instance, uint(i))
	}
	go bkr.invoker()
	return bkr, nil
}

func (bkr *BKR) computeAbaIds(participants []uuid.UUID) ([]uuid.UUID, error) {
	abaIds := make([]uuid.UUID, len(participants))
	for i, p := range participants {
		if id, err := bkr.computeAbaId(p); err != nil {
			return nil, fmt.Errorf("unable to compute aba id for participant %s: %w", id, err)
		} else {
			abaIds[i] = id
		}
	}
	bkrLogger.Info("computed aba ids", "ids", abaIds)
	return abaIds, nil
}

func (bkr *BKR) computeAbaId(participant uuid.UUID) (uuid.UUID, error) {
	if bkrId, err := bkr.id.MarshalBinary(); err != nil {
		return uuid.UUID{}, fmt.Errorf("unable to marshal BKR id: %w", err)
	} else if pId, err := participant.MarshalBinary(); err != nil {
		return uuid.UUID{}, fmt.Errorf("unable to marshal participant id: %w", err)
	} else {
		seed := append(bkrId, pId...)
		return utils.BytesToUUID(seed), nil
	}
}

func (bkr *BKR) deliverInput(input []byte, participant uuid.UUID) error {
	bkrLogger.Debug("scheduling input delivery", "input", string(input), "participant", participant)
	idx, err := bkr.getIdx(participant)
	if err != nil {
		return fmt.Errorf("unable to get index of participant: %w", err)
	} else if bkr.inputs[idx] != nil {
		return fmt.Errorf("input already delivered")
	}
	go func() {
		bkr.commands <- func() error {
			bkrLogger.Debug("received bkr input", "input", string(input), "participant", participant)
			bkr.inputs[idx] = input
			if bkr.canDeliver() {
				bkr.deliverOutput()
			}
			return bkr.tryToProposeToAba(idx, accept)
		}
	}()
	return nil
}

func (bkr *BKR) handleAbaResponse(instance *aba.AbaInstance, idx uint) {
	res := instance.GetOutput()
	bkrLogger.Info("received aba response", "idx", idx, "response", res)
	bkr.commands <- func() error {
		bkr.results[idx] = res
		if res == accept && lo.Count(bkr.results, accept) == len(bkr.participants)-int(bkr.f) {
			bkrLogger.Info("received threshold positive responses")
			err := bkr.rejectUnrespondingAbaInstances()
			if err != nil {
				return fmt.Errorf("unable to reject unresponding aba instances: %w", err)
			}
		}
		if bkr.canDeliver() {
			bkr.deliverOutput()
		}
		return nil
	}
}

func (bkr *BKR) canDeliver() bool {
	if bkr.hasDelivered || lo.CountBy(bkr.results, func(res byte) bool { return res != bot }) < len(bkr.participants) {
		return false
	}
	acceptedIndices := bkr.getAcceptedIndices()
	return lo.EveryBy(acceptedIndices, func(idx uint) bool { return bkr.inputs[idx] != nil }) // Received broadcast of all accepted ABA instances
}

func (bkr *BKR) deliverOutput() {
	acceptedIndices := bkr.getAcceptedIndices()
	slices.Sort(acceptedIndices)
	acceptedInputs := lo.Map(acceptedIndices, func(i uint, _ int) []byte { return bkr.inputs[i] })
	bkrLogger.Info("delivering output", "indices", acceptedIndices, "output", lo.Map(acceptedInputs, func(input []byte, _ int) string { return string(input) }))
	bkr.output <- acceptedInputs
}

func (bkr *BKR) getAcceptedIndices() []uint {
	return lo.FilterMap(bkr.results, func(res byte, idx int) (uint, bool) { return uint(idx), res == accept })
}

func (bkr *BKR) rejectUnrespondingAbaInstances() error {
	unresponsiveIdx := lo.FilterMap(bkr.results, func(res byte, idx int) (uint, bool) { return uint(idx), res == bot })
	bkrLogger.Info("rejecting unresponsive indexes", "idx", unresponsiveIdx)
	for _, idx := range unresponsiveIdx {
		go func() {
			bkr.commands <- func() error {
				return bkr.tryToProposeToAba(idx, reject)
			}
		}()
	}
	return nil
}

func (bkr *BKR) tryToProposeToAba(idx uint, val byte) error {
	if !bkr.iProposed[idx] {
		bkr.iProposed[idx] = true
		bkrLogger.Debug("proposing to aba", "idx", idx, "val", val)
		if err := bkr.abaInstances[idx].Propose(val); err != nil {
			return fmt.Errorf("unable to propose to aba: %w", err)
		}
	} else {
		bkrLogger.Debug("already proposed to aba", "idx", idx)
	}
	return nil
}

func (bkr *BKR) getIdx(participant uuid.UUID) (uint, error) {
	for i, p := range bkr.participants {
		if p == participant {
			return uint(i), nil
		}
	}
	return 0, fmt.Errorf("participant not found")
}

func (bkr *BKR) invoker() {
	for {
		select {
		case cmd := <-bkr.commands:
			if err := cmd(); err != nil {
				bkrLogger.Warn("unable to execute command", "error", err)
			}
		case <-bkr.closeChan:
			bkrLogger.Info("closing BKR invoker")
			return
		}
	}
}

func (bkr *BKR) Close() {
	bkrLogger.Info("sending closing BKR signal")
	bkr.closeChan <- struct{}{}
}
