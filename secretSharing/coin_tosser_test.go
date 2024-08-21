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

func TestAllSeeTheSameCoinToss(t *testing.T) {
	g := group.Ristretto255
	nodes := uint(4)
	threshold := uint(2)
	secret := g.NewScalar().SetUint64(1234567890)
	base := g.HashToElement([]byte("base"), []byte("test_coin"))
	controlToss, err := HashPointToBool(g.NewElement().Mul(base, secret))
	require.NoError(t, err)
	fmt.Println(controlToss)
	shares := ShareSecret(threshold, nodes, secret)
	pointSecret := g.Identity().Mul(base, secret)
	hiddenShares := lo.Map(shares, func(share secretsharing.Share, _ int) PointShare { return ShareToPoint(share, base) })
	recoveredSecret := RecoverSecretFromPoints(hiddenShares[:])
	fmt.Println("recovered secret", recoveredSecret)
	ptSecEnc, err := pointSecret.MarshalBinary()
	require.NoError(t, err)
	recSecEnc, err := recoveredSecret.MarshalBinary()
	require.Equal(t, ptSecEnc, recSecEnc)

	deals := makeDeals(shares)
	coinTossings := lo.Map(deals, func(deal *Deal, _ int) CoinToss {
		return *newCoinToss(uuid.New(), threshold, base, deal)
	})
	obs := mockCoinObs{make(chan bool)}
	coinTossings[0].AttachObserver(&obs)
	coinShares := lo.Map(coinTossings, func(coinToss CoinToss, _ int) PointShare {
		share, err := coinToss.tossCoin()
		require.NoError(t, err)
		return share
	})

	for _, coinShare := range coinShares {
		err = coinTossings[0].processShare(coinShare)
		require.NoError(t, err)
	}
	toss := <-obs.channel
	fmt.Printf("observing coin toss: %v\n", toss)
	require.Equal(t, toss, controlToss)
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
