package secretSharing

import (
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	. "github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockCoinObs struct {
	channel chan bool
}

func (m *mockCoinObs) observeCoin(_ UUID, toss bool) {
	m.channel <- toss
}

func TestAllSeeSameCoinToss(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(4)
	threshold := uint(2)
	secret := g.NewScalar().SetUint64(1234567890)
	shares := ShareSecret(threshold, nodes, secret)
	_, deals := makeDeals(shares)
	for i := 0; i < 100; i++ {
		seed := fmt.Sprintf("base_%d", i)
		base := g.HashToElement([]byte(seed), []byte("test_coin"))
		controlToss, err := HashPointToBool(g.NewElement().Mul(base, secret))
		require.NoError(t, err)
		coinTossings := lo.Map(deals, func(deal *Deal, _ int) coinToss {
			return *newCoinToss(New(), threshold, base, *deal)
		})
		obs := mockCoinObs{make(chan bool)}
		coinTossings[0].AttachObserver(&obs)
		coinShares := lo.Map(coinTossings, func(coinToss coinToss, _ int) coinTossShare {
			ct, err := coinToss.tossCoin()
			require.NoError(t, err)
			return ct
		})
		for _, coinShare := range coinShares {
			err = coinTossings[0].processShare(coinShare.ptShare)
			require.NoError(t, err)
		}
		toss := <-obs.channel
		require.Equal(t, toss, controlToss)
	}
}

func TestAllSeeSameCoinTossWithSerialization(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(4)
	threshold := uint(2)
	secret := g.NewScalar().SetUint64(1234567890)
	shares := ShareSecret(threshold, nodes, secret)
	pkIds, deals := makeDeals(shares)
	for i := 0; i < 100; i++ {
		seed := fmt.Sprintf("base_%d", i)
		base := g.HashToElement([]byte(seed), []byte("test_coin"))
		controlToss, err := HashPointToBool(g.NewElement().Mul(base, secret))
		require.NoError(t, err)
		coinTossings := lo.Map(deals, func(deal *Deal, _ int) coinToss {
			return *newCoinToss(New(), threshold, base, *deal)
		})
		obs := mockCoinObs{make(chan bool)}
		coinTossings[0].AttachObserver(&obs)
		coinShares := lo.Map(coinTossings, func(coinToss coinToss, _ int) coinTossShare {
			ct, err := coinToss.tossCoin()
			require.NoError(t, err)
			return ct
		})
		for _, tuple := range lo.Zip2(pkIds, coinShares) {
			pk, coinShare := tuple.Unpack()
			shareBytes, err := marshalCoinTossShare(coinShare)
			err = coinTossings[0].getShare(shareBytes, pk)
			require.NoError(t, err)
		}
		toss := <-obs.channel
		require.Equal(t, toss, controlToss)
	}
}

func makeDeals(shares []secretsharing.Share) ([]UUID, []*Deal) {
	commitBase := group.Ristretto255.HashToElement([]byte("commit"), []byte("test_coin"))
	pkIds, peerCommits := makePeerCommitments(shares, commitBase)
	return pkIds, lo.Map(shares, func(share secretsharing.Share, _ int) *Deal {
		commit := group.Ristretto255.NewElement().Mul(commitBase, share.Value)
		return &Deal{
			share:       share,
			commitBase:  commitBase,
			commit:      commit,
			peerCommits: peerCommits,
		}
	})
}

func makePeerCommitments(shares []secretsharing.Share, commitBase group.Element) ([]UUID, map[UUID]group.Element) {
	pkIds := lo.Map(shares, func(_ secretsharing.Share, _ int) UUID {
		id, _ := NewRandom()
		return id
	})
	peerCommitList := lo.Map(shares, func(share secretsharing.Share, i int) group.Element {
		return mulPoint(commitBase, share.Value)
	})
	peerCommits := make(map[UUID]group.Element)
	for _, tuple := range lo.Zip2(pkIds, peerCommitList) {
		id, commit := tuple.Unpack()
		peerCommits[id] = commit
	}
	return pkIds, peerCommits
}
