package agreementCommonSubset

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/mo"
	"log/slog"
	aba "pace/asynchronousBinaryAgreement"
	"pace/utils"
	"sync"
)

var proposalLogger = utils.GetLogger("Proposal Acceptor", slog.LevelDebug)

type proposalAcceptor struct {
	proposer  uuid.UUID
	aba       *aba.AbaInstance
	input     mo.Option[[]byte]
	inputLock sync.Mutex
	inputChan chan []byte
	proposed  bool
	output    chan mo.Option[[]byte]
}

func newProposalAcceptor(abaId uuid.UUID, proposer uuid.UUID, abaChan *aba.AbaChannel) *proposalAcceptor {
	abaInstance := abaChan.NewAbaInstance(abaId)
	p := &proposalAcceptor{
		proposer:  proposer,
		aba:       abaInstance,
		input:     mo.None[[]byte](),
		inputLock: sync.Mutex{},
		proposed:  false,
		inputChan: make(chan []byte, 1),

		output: make(chan mo.Option[[]byte], 1),
	}
	go p.waitResponse()
	proposalLogger.Info("new proposal acceptor created", "instance", abaId)
	return p
}

func (p *proposalAcceptor) submitInput(input []byte) error {
	p.inputLock.Lock()
	defer p.inputLock.Unlock()
	if p.input.IsPresent() {
		return fmt.Errorf("input already submitted")
	}
	proposalLogger.Info("submitting input", "input", input)
	p.input = mo.Some(input)
	p.inputChan <- input
	if !p.proposed {
		p.proposed = true
		if err := p.aba.Propose(accept); err != nil {
			return fmt.Errorf("unable to accpet input: %w", err)
		}
	}
	return nil
}

func (p *proposalAcceptor) rejectProposal() error {
	p.inputLock.Lock()
	defer p.inputLock.Unlock()
	if p.input.IsPresent() {
		return fmt.Errorf("input already submitted")
	}
	proposalLogger.Info("rejecting proposal")
	if !p.proposed {
		p.proposed = true
		if err := p.aba.Propose(reject); err != nil {
			return fmt.Errorf("unable to reject proposal: %w", err)
		}
	}
	return nil
}

func (p *proposalAcceptor) hasProposed() bool {
	p.inputLock.Lock()
	defer p.inputLock.Unlock()
	return p.proposed
}

func (p *proposalAcceptor) waitResponse() {
	res := p.aba.GetOutput()
	proposalLogger.Info("received decision", "decision", res)
	if res == accept {
		acceptedProposal := <-p.inputChan
		proposalLogger.Info("accepted proposal", "proposal", acceptedProposal)
		p.output <- mo.Some(acceptedProposal)
	} else {
		proposalLogger.Info("rejected proposal")
		p.output <- mo.None[[]byte]()
	}
}
