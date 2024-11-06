package asynchronousBinaryAgreement

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"math/rand/v2"
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
	testNetworkedInstancesShouldDecide(t, 1, 0)
}

func TestAllHonestNetworkedInstancesShouldDecide(t *testing.T) {
	testNetworkedInstancesShouldDecide(t, 10, 0)
}

func TestNetworkedInstancesShouldDecideWithFaults(t *testing.T) {
	f := uint(3)
	n := 3*f + 1
	testNetworkedInstancesShouldDecide(t, n, f)
}

func testNetworkedInstancesShouldDecide(t *testing.T, n, f uint) {
	nodes := lo.Map(lo.Range(int(n)), func(i int, _ int) *on.Node {
		address := fmt.Sprintf("localhost:%d", 6000+i)
		return on.GetNode(t, address, "localhost:6000")
	})
	ssChans := lo.Map(nodes, func(node *on.Node, _ int) *on.SSChannel {
		ss, err := on.CreateSSChannel(node, 'a')
		assert.NoError(t, err)
		return ss
	})
	on.InitializeNodes(t, nodes)
	id := uuid.New()
	abaInstances := lo.Map(nodes, func(node *on.Node, i int) *abaNetworkedInstance {
		abaInstance := makeAbaNetworkedInstance(t, id, node, ssChans[i], n, f)
		newNetworkedInstanceCommandIssuer(abaInstance)
		return abaInstance
	})
	assert.NoError(t, ct.DealSecret(ssChans[0], ct.NewScalar(42), f))
	proposals := lo.Map(lo.Range(int(n)), func(_ int, i int) byte { return byte(rand.IntN(2)) })
	for _, tuple := range lo.Zip2(abaInstances, proposals) {
		abaInstance, proposal := tuple.Unpack()
		assert.NoError(t, abaInstance.propose(proposal))
	}
	results := lo.Map(abaInstances, func(abaInstance *abaNetworkedInstance, _ int) byte { return <-abaInstance.output })
	assert.True(t, lo.EveryBy(results, func(res byte) bool { return res == results[0] }))
	assert.True(t, lo.EveryBy(nodes, func(node *on.Node) bool { return node.Disconnect() == nil }))
}

func makeAbaNetworkedInstance(t *testing.T, id uuid.UUID, node *on.Node, ssChan *on.SSChannel, n uint, f uint) *abaNetworkedInstance {
	ctBebChan := on.CreateBEBChannel(node, 'b')
	ctChan, err := ct.NewCoinTosserChannel(ssChan, ctBebChan, f)
	assert.NoError(t, err)
	abaBebChan := on.CreateBEBChannel(node, 'c')
	abamidware := newABAMiddleware(abaBebChan)
	termBebChan := on.CreateBEBChannel(node, 'd')
	termBrbChan := brb.CreateBRBChannel(n, f, termBebChan)
	termidware := newTerminationMiddleware(termBrbChan)
	abaInstance := newAbaNetworkedInstance(id, n, f, abamidware, termidware, ctChan)
	return abaInstance
}
