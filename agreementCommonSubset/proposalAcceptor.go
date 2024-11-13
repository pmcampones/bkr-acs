package agreementCommonSubset

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/mo"
	aba "pace/asynchronousBinaryAgreement"
	"sync"
)

type proposalAcceptor struct {
	proposer  uuid.UUID
	aba       *aba.AbaInstance
	input     mo.Option[[]byte]
	inputLock sync.Mutex
	inputChan chan []byte
	output    chan mo.Option[[]byte]
}

func newProposalAcceptor(abaId uuid.UUID, proposer uuid.UUID, abaChan *aba.AbaChannel) *proposalAcceptor {
	abaInstance := abaChan.NewAbaInstance(abaId)
	p := &proposalAcceptor{
		proposer:  proposer,
		aba:       abaInstance,
		input:     mo.None[[]byte](),
		inputLock: sync.Mutex{},
		inputChan: make(chan []byte, 1),
		output:    make(chan mo.Option[[]byte], 1),
	}
	go p.waitResponse()
	return p
}

func (p *proposalAcceptor) submitInput(input []byte) error {
	p.inputLock.Lock()
	defer p.inputLock.Unlock()
	if p.input.IsPresent() {
		return fmt.Errorf("input already submitted")
	}
	p.input = mo.Some(input)
	p.inputChan <- input
	if err := p.aba.Propose(accept); err != nil {
		return fmt.Errorf("unable to accpet input: %w", err)
	}
	return nil
}

func (p *proposalAcceptor) rejectProposal() error {
	p.inputLock.Lock()
	defer p.inputLock.Unlock()
	if p.input.IsPresent() {
		return fmt.Errorf("input already submitted")
	}
	if err := p.aba.Propose(reject); err != nil {
		return fmt.Errorf("unable to reject proposal: %w", err)
	}
	return nil
}

func (p *proposalAcceptor) hasProposed() bool {
	p.inputLock.Lock()
	defer p.inputLock.Unlock()
	return p.input.IsPresent()
}

func (p *proposalAcceptor) waitResponse() {
	res := p.aba.GetOutput()
	if res == accept {
		acceptedProposal := <-p.inputChan
		p.output <- mo.Some(acceptedProposal)
	} else {
		p.output <- mo.None[[]byte]()
	}
}
