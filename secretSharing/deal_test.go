package secretSharing

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net"
	"pace/network"
	"testing"
)

var dealCode byte = 'D'

func TestDeals(t *testing.T) {
	dealer, memObs, obs0, err := makeDealNode("localhost:6000", "localhost:6000", 2, 1)
	require.NoError(t, err)
	_, _, obs1, err := makeDealNode("localhost:6001", "localhost:6000", 2, 1)
	require.NoError(t, err)
	_, _, obs2, err := makeDealNode("localhost:6002", "localhost:6000", 2, 1)
	require.NoError(t, err)
	<-memObs.UpBarrier
	<-memObs.UpBarrier
	peers := memObs.Peers
	connList := make([]net.Conn, 0, len(peers))
	for _, peer := range peers {
		connList = append(connList, peer.Conn)
	}
	err = ShareDeals(1, dealer, connList, dealCode, obs0)
	deal0 := <-obs0.DealChan
	require.NoError(t, err)
	require.NotNil(t, deal0)
	fmt.Println("deal0", deal0)
	deal1 := <-obs1.DealChan
	require.NoError(t, err)
	require.NotNil(t, deal1)
	fmt.Println("deal1", deal1)
	deal2 := <-obs2.DealChan
	require.NoError(t, err)
	require.NotNil(t, deal2)
	fmt.Println("deal2", deal2)
}

func makeDealNode(address, contact string, bufferMsg, bufferMem int) (*network.Node, *network.TestMemObserver, *DealObserver, error) {
	node, memObs, _, err := network.MakeNode(address, contact, bufferMsg, bufferMem)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to create node: %v", err)
	}
	dealObs := &DealObserver{
		Code:     dealCode,
		DealChan: make(chan *Deal),
	}
	node.AttachMessageObserver(dealObs)
	return node, memObs, dealObs, nil
}
