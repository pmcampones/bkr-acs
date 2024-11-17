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

var bkrLogger = utils.GetLogger("BKR Instance", slog.LevelDebug)

const (
	accept = 1
	reject = 0
)

type bkr struct {
	id          uuid.UUID
	f           uint
	acceptors   []*proposalAcceptor
	resultsChan chan lo.Tuple2[mo.Option[[]byte], uint]
	results     [][]byte
	output      chan [][]byte
}

func newBKR(id uuid.UUID, f uint, proposers []uuid.UUID, abaChan *aba.AbaChannel) *bkr {
	bkrLogger.Info("initializing bkr", "id", id, "f", f, "proposers", proposers)
	b := &bkr{
		id:          id,
		f:           f,
		acceptors:   computeAcceptors(id, proposers, abaChan),
		resultsChan: make(chan lo.Tuple2[mo.Option[[]byte], uint], len(proposers)),
		results:     make([][]byte, len(proposers)),
		output:      make(chan [][]byte, 1),
	}
	go b.processResponses()
	for i, acceptor := range b.acceptors {
		go b.waitAcceptorResponse(acceptor, uint(i))
	}
	return b
}

func computeAcceptors(bkrId uuid.UUID, proposers []uuid.UUID, abaChan *aba.AbaChannel) []*proposalAcceptor {
	return lo.Map(proposers, func(proposer uuid.UUID, _ int) *proposalAcceptor {
		abaId := utils.BytesToUUID(append(bkrId[:], proposer[:]...))
		return newProposalAcceptor(abaId, proposer, abaChan)
	})
}

func (b *bkr) waitAcceptorResponse(acceptor *proposalAcceptor, idx uint) {
	response := <-acceptor.output
	b.resultsChan <- lo.Tuple2[mo.Option[[]byte], uint]{A: response, B: idx}
}

func (b *bkr) processResponses() {
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

func (b *bkr) getAccepted() [][]byte {
	return lo.Filter(b.results, func(r []byte, _ int) bool { return r != nil })
}

func (b *bkr) receiveInput(input []byte, proposer uuid.UUID) error {
	bkrLogger.Debug("receiving input", "input", string(input), "proposer", proposer)
	acceptor, err := b.getAcceptor(proposer)
	if err != nil {
		return fmt.Errorf("unable to find acceptor for proposer %s", proposer)
	}
	return acceptor.submitInput(input)
}

func (b *bkr) getAcceptor(proposer uuid.UUID) (*proposalAcceptor, error) {
	for _, a := range b.acceptors {
		if a.proposer == proposer {
			return a, nil
		}
	}
	return nil, fmt.Errorf("unable to find acceptor for proposer %s", proposer)
}
