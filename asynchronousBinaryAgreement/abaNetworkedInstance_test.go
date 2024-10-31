package asynchronousBinaryAgreement

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	brb "pace/byzantineReliableBroadcast"
	ct "pace/coinTosser"
	on "pace/overlayNetwork"
	"testing"
)

type networkedInstanceCommandIssuer struct {
	aba *abaNetworkedInstance
}

func newNetworkedInstanceCommandIssuer(aba *abaNetworkedInstance) *networkedInstanceCommandIssuer {
	n := &networkedInstanceCommandIssuer{
		aba: aba,
	}
	go n.issueReceivedCommands()
	return n
}

func (n *networkedInstanceCommandIssuer) issueReceivedCommands() {
	for {
		select {
		case term := <-n.aba.termidware.output:
			n.aba.submitDecision(term.decision, term.sender)
		case abamsg := <-n.aba.abamidware.output:
			n.issueAbaMsgCommand(abamsg)
		}
	}
}

func (n *networkedInstanceCommandIssuer) issueAbaMsgCommand(abamsg *abaMsg) {
	switch abamsg.kind {
	case bval:
		n.aba.submitBVal(abamsg.val, abamsg.sender, abamsg.round)
	case aux:
		n.aba.submitAux(abamsg.val, abamsg.sender, abamsg.round)
	}
}

func TestNetworkedInstanceShouldDecideOwnProposal(t *testing.T) {
	node := on.GetNode(t, "localhost:6000", "localhost:6000")
	ssChan, err := on.CreateSSChannel(node, 'a')
	assert.NoError(t, err)
	ctBebChan := on.CreateBEBChannel(node, 'b')
	ctChan, err := ct.NewCoinTosserChannel(ssChan, ctBebChan, 0)
	assert.NoError(t, err)
	abaBebChan := on.CreateBEBChannel(node, 'c')
	abamidware := newABAMiddleware(abaBebChan)
	termBebChan := on.CreateBEBChannel(node, 'd')
	termBrbChan := brb.CreateBRBChannel(1, 0, termBebChan, make(chan brb.BRBMsg))
	termidware := newTerminationMiddleware(termBrbChan)
	on.InitializeNodes(t, []*on.Node{node})
	assert.NoError(t, ct.DealSecret(ssChan, ct.NewScalar(42), 0))
	abaInstance := newAbaNetworkedInstance(uuid.New(), 1, 0, abamidware, termidware, ctChan)
	newNetworkedInstanceCommandIssuer(abaInstance)
	assert.NoError(t, abaInstance.propose(1))
	res := <-abaInstance.output
	assert.Equal(t, byte(1), res)
}
