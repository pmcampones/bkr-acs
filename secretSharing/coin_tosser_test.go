package secretSharing

import (
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockCoinObs struct {
	channel chan bool
}

func (m *mockCoinObs) observeCoin(_ uuid.UUID, toss bool) {
	m.channel <- toss
}

func TestAllSeeSameCoinToss(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(4)
	threshold := uint(2)
	secret := g.NewScalar().SetUint64(1234567890)
	shares := ShareSecret(threshold, nodes, secret)
	deals := makeDeals(shares)
	for i := 0; i < 100; i++ {
		seed := fmt.Sprintf("base_%d", i)
		base := g.HashToElement([]byte(seed), []byte("test_coin"))
		controlToss, err := HashPointToBool(g.NewElement().Mul(base, secret))
		require.NoError(t, err)
		coinTossings := lo.Map(deals, func(deal *Deal, _ int) CoinToss {
			return *newCoinToss(uuid.New(), threshold, base, deal)
		})
		obs := mockCoinObs{make(chan bool)}
		coinTossings[0].AttachObserver(&obs)
		coinShares := lo.Map(coinTossings, func(coinToss CoinToss, _ int) PointShare { return coinToss.tossCoin() })
		for _, coinShare := range coinShares {
			err = coinTossings[0].processShare(coinShare)
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
	deals := makeDeals(shares)
	for i := 0; i < 100; i++ {
		seed := fmt.Sprintf("base_%d", i)
		base := g.HashToElement([]byte(seed), []byte("test_coin"))
		controlToss, err := HashPointToBool(g.NewElement().Mul(base, secret))
		require.NoError(t, err)
		coinTossings := lo.Map(deals, func(deal *Deal, _ int) CoinToss {
			return *newCoinToss(uuid.New(), threshold, base, deal)
		})
		obs := mockCoinObs{make(chan bool)}
		coinTossings[0].AttachObserver(&obs)
		coinShares := lo.Map(coinTossings, func(coinToss CoinToss, _ int) PointShare { return coinToss.tossCoin() })
		for _, coinShare := range coinShares {
			shareBytes, err := marshalPointShare(coinShare)
			err = coinTossings[0].getShare(shareBytes)
			require.NoError(t, err)
		}
		toss := <-obs.channel
		require.Equal(t, toss, controlToss)
	}
}

func makeDeals(shares []secretsharing.Share) []*Deal {
	commitBase := group.Ristretto255.HashToElement([]byte("commit"), []byte("test_coin"))
	return lo.Map(shares, func(share secretsharing.Share, _ int) *Deal {
		commit := group.Ristretto255.NewElement().Mul(commitBase, share.Value)
		return &Deal{
			share:      &share,
			commitBase: &commitBase,
			commit:     &commit,
		}
	})
}
