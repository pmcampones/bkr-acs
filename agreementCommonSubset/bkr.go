package agreementCommonSubset

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	aba "pace/asynchronousBinaryAgreement"
	"pace/utils"
	"slices"
)

const (
	accept = 1
	reject = 0
)

type BKR struct {
	id           uuid.UUID
	f            uint
	participants []uuid.UUID
	iProposed    map[uuid.UUID]bool
	inputs       [][]byte
	results      []lo.Tuple2[uint, byte]
	abaChannel   *aba.AbaChannel
	output       chan [][]byte
	commands     chan func() error
	closeChan    chan struct{}
}

func newBKR(id uuid.UUID, f uint, participants []uuid.UUID, abaChan *aba.AbaChannel) (*BKR, error) {
	bkr := &BKR{
		id:           id,
		f:            f,
		participants: participants,
		iProposed:    make(map[uuid.UUID]bool),
		inputs:       make([][]byte, len(participants)),
		results:      make([]lo.Tuple2[uint, byte], 0),
		abaChannel:   abaChan,
		output:       make(chan [][]byte, 1),
		commands:     make(chan func() error),
		closeChan:    make(chan struct{}, 1),
	}
	abaIds, err := bkr.computeAbaIds(participants)
	if err != nil {
		return nil, fmt.Errorf("unable to compute aba ids: %w", err)
	}
	abaReturnChans := lo.Map(abaIds, func(abaId uuid.UUID, _ int) chan byte { return bkr.abaChannel.NewAbaInstance(abaId) })
	for i, returnChan := range abaReturnChans {
		go bkr.handleAbaResponse(returnChan, uint(i))
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
	idx, err := bkr.getIdx(participant)
	if err != nil {
		return fmt.Errorf("unable to get index of participant: %w", err)
	} else if bkr.inputs[idx] != nil {
		return fmt.Errorf("input already delivered")
	}
	bkr.commands <- func() error {
		bkr.inputs[idx] = input
		if bkr.isFinished() {
			bkr.deliverOutput()
		}
		abaId, err := bkr.computeAbaId(participant)
		if err != nil {
			return fmt.Errorf("unable to compute aba id: %w", err)
		}
		bkr.tryToProposeToAba(abaId, accept)
		return nil
	}
	return nil
}

func (bkr *BKR) handleAbaResponse(returnChan chan byte, idx uint) {
	res := <-returnChan
	bkr.commands <- func() error {
		bkr.results = append(bkr.results, lo.Tuple2[uint, byte]{A: idx, B: res})
		if res == 1 && bkr.countPositive() == uint(len(bkr.participants))-bkr.f {
			err := bkr.rejectUnrespondingAbaInstances()
			if err != nil {
				return fmt.Errorf("unable to reject unresponding aba instances: %w", err)
			}
		}
		if bkr.isFinished() {
			bkr.deliverOutput()
		}
		return nil
	}
}

func (bkr *BKR) isFinished() bool {
	if len(bkr.results) != len(bkr.participants) { // Not all ABA instances finished
		return false
	}
	acceptedIndices := bkr.getAcceptedIndices()
	return lo.EveryBy(acceptedIndices, func(idx uint) bool { return bkr.inputs[idx] != nil }) // Received broadcast of all accepted ABA instances
}

func (bkr *BKR) deliverOutput() {
	acceptedIndices := bkr.getAcceptedIndices()
	slices.Sort(acceptedIndices)
	acceptedInputs := lo.Map(acceptedIndices, func(i uint, _ int) []byte { return bkr.inputs[i] })
	bkr.output <- acceptedInputs
}

func (bkr *BKR) getAcceptedIndices() []uint {
	acceptedTuples := lo.Filter(bkr.results, func(res lo.Tuple2[uint, byte], _ int) bool { return res.B == 1 })
	acceptedIndices := lo.Map(acceptedTuples, func(res lo.Tuple2[uint, byte], _ int) uint { return res.A })
	return acceptedIndices
}

func (bkr *BKR) rejectUnrespondingAbaInstances() error {
	unresponsiveIds, err := bkr.computeUnresponsiveIds()
	if err != nil {
		return fmt.Errorf("unable to compute unresponsive ids: %w", err)
	}
	for _, id := range unresponsiveIds {
		go func() {
			bkr.commands <- func() error {
				bkr.tryToProposeToAba(id, reject)
				return nil
			}
		}()
	}
	return nil
}

func (bkr *BKR) tryToProposeToAba(abaId uuid.UUID, val byte) {
	if !bkr.iProposed[abaId] {
		bkr.iProposed[abaId] = true
		bkr.abaChannel.Propose(abaId, val)
	}
}

func (bkr *BKR) countInputs() uint {
	return uint(lo.CountBy(bkr.inputs, func(input []byte) bool { return input != nil }))
}

func (bkr *BKR) countPositive() uint {
	vals := lo.Map(bkr.results, func(res lo.Tuple2[uint, byte], _ int) byte { return res.B })
	return uint(lo.Count(vals, 1))
}

func (bkr *BKR) computeUnresponsiveIds() ([]uuid.UUID, error) {
	responseIdx := lo.Map(bkr.results, func(res lo.Tuple2[uint, byte], _ int) uint { return res.A })
	mapResponseIdx := make(map[uint]bool)
	for _, idx := range responseIdx {
		mapResponseIdx[idx] = true
	}
	unresponsiveIdx := lo.Filter(lo.Range(len(bkr.participants)), func(i int, _ int) bool { return !mapResponseIdx[uint(i)] })
	unresponsiveParticipants := lo.Map(unresponsiveIdx, func(i int, _ int) uuid.UUID { return bkr.participants[i] })
	unresponsiveAba, err := bkr.computeAbaIds(unresponsiveParticipants)
	if err != nil {
		return nil, fmt.Errorf("unable to compute aba ids for unresponsive participants: %w", err)
	}
	return unresponsiveAba, nil
}

func (bkr *BKR) getIdx(participant uuid.UUID) (int, error) {
	for i, p := range bkr.participants {
		if p == participant {
			return i, nil
		}
	}
	return -1, fmt.Errorf("participant not found")
}

func (bkr *BKR) invoker() {
	for {
		select {
		case cmd := <-bkr.commands:
			err := cmd()
			if err != nil {
				fmt.Println(err)
			}
		case <-bkr.closeChan:
			return
		}
	}
}

func (bkr *BKR) Close() {
	bkr.closeChan <- struct{}{}
}
