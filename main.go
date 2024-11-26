package main

import (
	acs "bkr-acs/agreementCommonSubset"
	aba "bkr-acs/asynchronousBinaryAgreement"
	brb "bkr-acs/byzantineReliableBroadcast"
	ct "bkr-acs/coinTosser"
	on "bkr-acs/overlayNetwork"
	"bkr-acs/utils"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/magiconair/properties"
	"github.com/samber/lo"
	"log/slog"
	"slices"
)

var logger = utils.GetLogger("Main", slog.LevelDebug)

func main() {
	propsPathname := flag.String("config", "config/config.properties", "pathname of the configuration file")
	address := flag.String("address", "localhost:6000", "address of the current node")
	flag.Parse()
	props := properties.MustLoadFile(*propsPathname, properties.UTF8)
	logger.Info("loaded properties", allPropertiesList(props)...)
	contact := props.MustGetString("contact")
	node, err := on.NewNode(*address, contact)
	if err != nil {
		panic(fmt.Errorf("unable to create node: %v", err))
	}
	logger.Info("node created", "address", *address, "contact", contact)
	bkrChannel, err := computeBkrChannel(props, node, *address == contact)
	if err != nil {
		panic(fmt.Errorf("unable to create bkr channel: %v", err))
	}
	if participateBKR(bkrChannel) != nil {
		panic(fmt.Errorf("error while participating in bkr: %v", err))
	}
}

func allPropertiesList(props *properties.Properties) []any {
	allProps := make([]any, 0, 2*len(props.Map()))
	for key, val := range props.Map() {
		allProps = append(allProps, key)
		allProps = append(allProps, val)
	}
	return allProps
}

func computeBkrChannel(props *properties.Properties, node *on.Node, amContact bool) (*acs.BKRChannel, error) {
	numNodes := props.MustGetUint("num_nodes")
	faulty := props.MustGetUint("faulty")
	dealCode := props.MustGetString("deal_code")[0]
	dealSS := on.NewSSChannel(node, dealCode)
	ctCode := props.MustGetString("ct_code")[0]
	ctBeb := on.NewBEBChannel(node, ctCode)
	abaCode := props.MustGetString("aba_code")[0]
	abaBeb := on.NewBEBChannel(node, abaCode)
	tCode := props.MustGetString("t_code")[0]
	tBeb := on.NewBEBChannel(node, tCode)
	bkrCode := props.MustGetString("bkr_code")[0]
	bkrBeb := on.NewBEBChannel(node, bkrCode)
	bkrBrb := brb.NewBRBChannel(numNodes, faulty, bkrBeb)
	if node.Join() != nil {
		return nil, fmt.Errorf("unable to join the network")
	}
	logger.Info("node joined the network and is waiting for peers", "numNodes", numNodes)
	node.WaitForPeers(numNodes - 1)
	logger.Info("network is stable")
	if amContact {
		if err := ct.DealSecret(dealSS, ct.RandomScalar(), faulty); err != nil {
			return nil, fmt.Errorf("unable to deal secret: %v", err)
		}
	}
	abaChannel, err := aba.NewAbaChannel(numNodes, faulty, dealSS, ctBeb, abaBeb, tBeb)
	if err != nil {
		return nil, fmt.Errorf("unable to create aba channel: %v", err)
	}
	participants, err := getParticipantIds(node)
	if err != nil {
		return nil, fmt.Errorf("unable to get participant ids: %v", err)
	}
	return acs.NewBKRChannel(faulty, abaChannel, bkrBrb, participants), nil
}

func getParticipantIds(node *on.Node) ([]uuid.UUID, error) {
	unsortedIds, err := node.GetPeerIds()
	if err != nil {
		return nil, fmt.Errorf("unable to get peer unsortedIds: %v", err)
	}
	id, err := node.GetId()
	if err != nil {
		return nil, fmt.Errorf("unable to get my id: %v", err)
	}
	unsortedIds = append(unsortedIds, id)
	idsString := lo.Map(unsortedIds, func(id uuid.UUID, _ int) string {
		return id.String()
	})
	slices.Sort(idsString)
	sortedIds := lo.Map(idsString, func(id string, _ int) uuid.UUID {
		return uuid.MustParse(id)
	})
	return sortedIds, nil
}

func participateBKR(bkrChannel *acs.BKRChannel) error {
	for i := 0; ; i++ {
		fmt.Printf("Input for BKR instance %d:\n", i)
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			return fmt.Errorf("unable to read user input: %v", err)
		}
		id := utils.BytesToUUID([]byte(fmt.Sprintf("bkr-%d", i)))
		proposal := []byte(input)
		outputChan := bkrChannel.NewBKRInstance(id)
		if err := bkrChannel.Propose(id, proposal); err != nil {
			return fmt.Errorf("unable to propose val: %v", err)
		}
		output := <-outputChan
		fmt.Printf("Output for BKR instance %d:\n", i)
		for i, val := range output {
			fmt.Printf("%d: %s\n", i, string(val))
		}
	}
}
