package secretSharing

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"maps"
	"pace/network"
	"pace/utils"
	"slices"
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
	peers := slices.Collect(maps.Values(memObs.Peers))
	err = ShareDeals(1, dealer, peers, dealCode, obs0)
	deal0 := <-obs0.DealChan
	require.NoError(t, err)
	require.NotNil(t, deal0)
	fmt.Println("deal0", deal0)
	deal1 := <-obs1.DealChan
	require.NotNil(t, deal1)
	fmt.Println("deal1", deal1)
	deal2 := <-obs2.DealChan
	require.NotNil(t, deal2)
	fmt.Println("deal2", deal2)
	commits1 := deal1.peerCommits
	commits2 := deal2.peerCommits
	require.Equal(t, len(commits1), len(commits2))
	for pk, pt1 := range commits1 {
		pt2, ok := commits2[pk]
		require.True(t, ok)
		pt1Bytes, err := pt1.MarshalBinary()
		require.NoError(t, err)
		pt2Bytes, err := pt2.MarshalBinary()
		require.Equal(t, pt1Bytes, pt2Bytes)
	}
}

func TestSerializeAndDeserializeCommits(t *testing.T) {
	numCommits := 100
	points := make([]group.Element, numCommits)
	for i := 0; i < numCommits; i++ {
		seed := []byte(fmt.Sprintf("point_%d", i))
		points[i] = group.Ristretto255.HashToElement(seed, []byte("test_commit_serialize"))
	}
	pks := make([]ecdsa.PublicKey, numCommits)
	for i := 0; i < numCommits; i++ {
		pk, err := utils.GenPK()
		require.NoError(t, err)
		pks[i] = *pk
	}
	commits := lo.Zip2(points, pks)
	serialized, err := serializeCommitments(commits)
	require.NoError(t, err)
	deserialized, err := deserializeCommitments(serialized)
	require.NoError(t, err)
	recovPts, recovPKs := lo.Unzip2(deserialized)
	for i, ptTuple := range lo.Zip2(points, recovPts) {
		og, recov := ptTuple.Unpack()
		ogBytes, err := og.MarshalBinary()
		require.NoError(t, err)
		recovBytes, err := recov.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, ogBytes, recovBytes, "point %d", i)
	}
	for i, pkTuple := range lo.Zip2(pks, recovPKs) {
		og, recov := pkTuple.Unpack()
		require.Equal(t, og, recov, "pk %d", i)
	}
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
