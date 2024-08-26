package secretSharing

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"maps"
	"pace/utils"
	"slices"
	"testing"
	"time"
)

func TestTrue(t *testing.T) {
	threshold := uint(3)
	utils.SetupDefaultLogger()
	node0, memObs, dealObs0, err := makeDealNode("localhost:6000", "localhost:6000", 4, 4)
	require.NoError(t, err)
	node1, _, dealObs1, err := makeDealNode("localhost:6001", "localhost:6000", 4, 4)
	require.NoError(t, err)
	node2, _, dealObs2, err := makeDealNode("localhost:6002", "localhost:6000", 4, 4)
	require.NoError(t, err)
	node3, _, dealObs3, err := makeDealNode("localhost:6003", "localhost:6000", 4, 4)
	require.NoError(t, err)
	fmt.Println("Nodes created")
	<-memObs.UpBarrier
	<-memObs.UpBarrier
	<-memObs.UpBarrier
	fmt.Println("Contact connected to all")
	peers := slices.Collect(maps.Values(memObs.Peers))
	err = ShareDeals(3, node0, peers, dealCode, dealObs0)
	require.NoError(t, err)
	deal0 := <-dealObs0.DealChan
	deal1 := <-dealObs1.DealChan
	deal2 := <-dealObs2.DealChan
	deal3 := <-dealObs3.DealChan
	fmt.Println("Deals shared")
	ctChannel0 := NewCoinTosserChannel(node0, threshold, *deal0, 'C')
	ctChannel1 := NewCoinTosserChannel(node1, threshold, *deal1, 'C')
	ctChannel2 := NewCoinTosserChannel(node2, threshold, *deal2, 'C')
	ctChannel3 := NewCoinTosserChannel(node3, threshold, *deal3, 'C')
	time.Sleep(time.Second) // making sure the channels attach to the network to get updates
	fmt.Println("CT Channels created")
	ch0 := make(chan struct {
		bool
		error
	})
	ch1 := make(chan struct {
		bool
		error
	})
	ch2 := make(chan struct {
		bool
		error
	})
	ch3 := make(chan struct {
		bool
		error
	})
	ctChannel0.TossCoin([]byte("seed"), ch0)
	ctChannel1.TossCoin([]byte("seed"), ch1)
	ctChannel2.TossCoin([]byte("seed"), ch2)
	ctChannel3.TossCoin([]byte("seed"), ch3)
	fmt.Println("Coins tossed")
	coin0 := <-ch0
	require.NoError(t, coin0.error)
	coin1 := <-ch1
	require.NoError(t, coin1.error)
	coin2 := <-ch2
	require.NoError(t, coin2.error)
	coin3 := <-ch3
	require.NoError(t, coin3.error)
	fmt.Println("Coins received")
	require.Equal(t, coin0.bool, coin1.bool)
	require.Equal(t, coin1.bool, coin2.bool)
	require.Equal(t, coin2.bool, coin3.bool)
}
