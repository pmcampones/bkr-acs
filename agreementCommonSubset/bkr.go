package agreementCommonSubset

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"log/slog"
	aba "pace/asynchronousBinaryAgreement"
	"pace/utils"
)

var bkrLogger = utils.GetLogger(slog.LevelDebug)

const (
	accept = 1
	reject = 0
)

type BKR struct {
	id          uuid.UUID
	f           uint
	acceptors   []*proposalAcceptor
	resultsChan chan lo.Tuple2[mo.Option[[]byte], uint]
	results     [][]byte
	output      chan [][]byte
}

func NewBKR(id uuid.UUID, f uint, proposers []uuid.UUID, abaChan *aba.AbaChannel) *BKR {
	bkrLogger.Info("initializing BKR", "id", id, "f", f, "proposers", proposers)
	b2 := &BKR{
		id:          id,
		f:           f,
		acceptors:   computeAcceptors(id, proposers, abaChan),
		resultsChan: make(chan lo.Tuple2[mo.Option[[]byte], uint], len(proposers)),
		results:     make([][]byte, len(proposers)),
		output:      make(chan [][]byte, 1),
	}
	go b2.processResponses()
	for i, acceptor := range b2.acceptors {
		go b2.waitAcceptorResponse(acceptor, uint(i))
	}
	return b2
}

func computeAcceptors(bkrId uuid.UUID, proposers []uuid.UUID, abaChan *aba.AbaChannel) []*proposalAcceptor {
	return lo.Map(proposers, func(proposer uuid.UUID, _ int) *proposalAcceptor {
		abaId := utils.BytesToUUID(append(bkrId[:], proposer[:]...))
		return newProposalAcceptor(abaId, proposer, abaChan)
	})
}

func (b *BKR) waitAcceptorResponse(acceptor *proposalAcceptor, idx uint) {
	response := <-acceptor.output
	b.resultsChan <- lo.Tuple2[mo.Option[[]byte], uint]{A: response, B: idx}
}

func (b *BKR) processResponses() {
	for i := uint(0); i < uint(len(b.acceptors)); i++ {
		response := <-b.resultsChan
		proposal, idx := response.Unpack()
		bkrLogger.Info("processing response", "proposal", proposal.OrEmpty(), "idx", idx)
		b.results[idx] = proposal.OrEmpty()
		if proposal.IsPresent() && len(b.getAccepted()) == len(b.acceptors)-int(b.f) {
			bkrLogger.Info("trying to reject unresponding proposals")
			for i, a := range b.acceptors {
				if !a.hasProposed() {
					bkrLogger.Debug("rejecting proposal", "idx", i)
					if err := a.rejectProposal(); err != nil {
						bkrLogger.Warn("unable to reject proposal", "idx", i, "error", err)
					}
				}
			}
		}
	}
	accepted := b.getAccepted()
	bkrLogger.Info("outputting accepted proposals", "accepted", accepted)
	b.output <- accepted
}

func (b *BKR) getAccepted() [][]byte {
	return lo.Filter(b.results, func(r []byte, _ int) bool { return r != nil })
}

func (b *BKR) receiveInput(input []byte, proposer uuid.UUID) error {
	bkrLogger.Debug("receiving input", "input", string(input), "proposer", proposer)
	for _, a := range b.acceptors {
		if a.proposer == proposer {
			if !a.hasProposed() {
				return a.submitInput(input)
			}
			return nil
		}
	}
	return fmt.Errorf("unable to find acceptor for proposer %s", proposer)
}
