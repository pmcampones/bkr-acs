package coinTosser

import (
	"fmt"
	"github.com/magiconair/properties"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pace/overlayNetwork"
	"pace/utils"
	"testing"
)

// This test should work with threshold = 3 but does not. I don't know why but the broadcasts in some replicas aren't reaching all peers.
func TestTrue(t *testing.T) {
	setProperties(t)
	threshold := uint(3)
	ctChannels := setupNodes(t, int(threshold))
	err := ctChannels[0].ShareDeal()
	assert.NoError(t, err)
	coinReceivers := lo.Map(ctChannels, func(_ *CTChannel, _ int) chan mo.Result[bool] { return make(chan mo.Result[bool], 2) })
	fmt.Println("CT Channels created")
	for _, tuple := range lo.Zip2(ctChannels, coinReceivers) {
		ctChannel, coinRec := tuple.Unpack()
		ctChannel.TossCoin([]byte("seed"), coinRec)
	}
	fmt.Println("Coins tossed")
	coinsRes := make([]mo.Result[bool], 4)
	for i, coinRec := range coinReceivers {
		coinsRes[i] = <-coinRec
	}
	assert.True(t, lo.EveryBy(coinsRes, func(coin mo.Result[bool]) bool { return coin.IsOk() }))
	fmt.Println("Coins received")
	coins := lo.Map(coinsRes, func(coin mo.Result[bool], _ int) bool { return coin.MustGet() })
	assert.True(t, lo.EveryBy(coins, func(coin bool) bool { return coin == coins[0] }))
}

func setProperties(t *testing.T) {
	props := properties.NewProperties()
	_, _, err := props.Set("ct_code", "C")
	require.NoError(t, err)
	_, _, err = props.Set("deal_code", "D")
	require.NoError(t, err)
	err = utils.SetProps(props)
	require.NoError(t, err)
}

// setupNodes creates 4 nodes attached message listeners and connects them to each other.
func setupNodes(t *testing.T, threshold int) []*CTChannel {
	contact := "localhost:6000"
	_, memObs, ct0 := makeCTNode(t, "localhost:6000", contact, 4, 4, threshold)
	_, _, ct1 := makeCTNode(t, "localhost:6001", contact, 4, 4, threshold)
	_, _, ct2 := makeCTNode(t, "localhost:6002", contact, 4, 4, threshold)
	_, _, ct3 := makeCTNode(t, "localhost:6003", contact, 4, 4, threshold)
	fmt.Println("Nodes created")
	<-memObs.UpBarrier
	<-memObs.UpBarrier
	<-memObs.UpBarrier
	fmt.Println("Contact connected to all")
	ctChannels := []*CTChannel{ct0, ct1, ct2, ct3}
	return ctChannels
}

func makeCTNode(t *testing.T, address, contact string, bufferMsg, bufferMem, threshold int) (*overlayNetwork.Node, *overlayNetwork.TestMemObserver, *CTChannel) {
	node, memObs, _, err := overlayNetwork.MakeNode(address, contact, bufferMsg, bufferMem)
	assert.NoError(t, err)
	ctChannel := NewCoinTosserChannel(node, uint(threshold))
	//node.attachMessageObserver(ctChannel)
	return node, memObs, ctChannel
}
