package coinTosser

import (
	"crypto/rand"
	"github.com/cloudflare/circl/group"
	ss "github.com/cloudflare/circl/secretsharing"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldTossCoinToSelf(t *testing.T) {
	testShouldTossWithoutThreshold(t, 1)
}

func TestShouldAllSeeSameCoinNoThreshold(t *testing.T) {
	testShouldTossWithoutThreshold(t, 10)
}

func testShouldTossWithoutThreshold(t *testing.T, nodes uint) {
	threshold := uint(0)
	secret := NewScalar(42)
	deals := makeLocalDeals(threshold, nodes, secret)
	outputChans := lo.Map(deals, func(deal *deal, _ int) chan bool { return make(chan bool) })
	base := group.Ristretto255.HashToElement([]byte("base"), []byte("instance_tests"))
	blindedSecret := mulPoint(base, secret)
	coinTossings := lo.ZipBy2(deals, outputChans, func(d *deal, oc chan bool) *coinToss {
		return newCoinToss(threshold, base, d, oc)
	})
	coinShares := lo.Map(coinTossings, func(ct *coinToss, _ int) ctShare {
		c, err := ct.tossCoin()
		assert.NoError(t, err)
		return c
	})
	assert.True(t, lo.EveryBy(coinShares, func(cs ctShare) bool { return areElementsEqualsTest(t, cs.pt.point, blindedSecret) }))
	for _, tuple := range lo.Zip2(coinTossings, coinShares) {
		ct, cs := tuple.Unpack()
		err := ct.submitShare(cs, uuid.New())
		assert.NoError(t, err)
	}
	outcomes := lo.Map(outputChans, func(oc chan bool, _ int) bool {
		return <-oc
	})
	firstOutcome := outcomes[0]
	assert.True(t, lo.EveryBy(outcomes, func(outcome bool) bool { return outcome == firstOutcome }))
}

func TestShouldAllSeeSameCoinWithHalfThreshold(t *testing.T) {
	numNodes := uint(10)
	threshold := numNodes / 2
	testShouldAllSeeSameCoinWithThreshold(t, numNodes, threshold)
}

func TestShouldAllSeeSameCoinWithFullThreshold(t *testing.T) {
	numNodes := uint(10)
	threshold := numNodes - 1
	testShouldAllSeeSameCoinWithThreshold(t, numNodes, threshold)
}

func testShouldAllSeeSameCoinWithThreshold(t *testing.T, nodes, threshold uint) {
	secret := NewScalar(42)
	deals := makeLocalDeals(threshold, nodes, secret)
	outputChans := lo.Map(deals, func(deal *deal, _ int) chan bool { return make(chan bool) })
	base := group.Ristretto255.HashToElement([]byte("base"), []byte("instance_tests"))
	coinTossings := lo.ZipBy2(deals, outputChans, func(d *deal, oc chan bool) *coinToss {
		return newCoinToss(threshold, base, d, oc)
	})
	coinShares := lo.Map(coinTossings, func(ct *coinToss, _ int) ctShare {
		c, err := ct.tossCoin()
		assert.NoError(t, err)
		return c
	})
	for _, ct := range coinTossings {
		for _, cs := range coinShares {
			err := ct.submitShare(cs, uuid.New())
			assert.NoError(t, err)
		}
	}
	outcomes := lo.Map(outputChans, func(oc chan bool, _ int) bool {
		return <-oc
	})
	firstOutcome := outcomes[0]
	assert.True(t, lo.EveryBy(outcomes, func(outcome bool) bool { return outcome == firstOutcome }))
}

func makeLocalDeals(threshold, numNodes uint, secret group.Scalar) []*deal {
	shares := shareSecret(threshold, numNodes, secret)
	base := group.Ristretto255.RandomElement(rand.Reader)
	commits := lo.Map(shares, func(share ss.Share, _ int) pointShare {
		return shareToPoint(share, base)
	})
	return lo.Map(shares, func(share ss.Share, i int) *deal {
		return &deal{
			share:   share,
			base:    base,
			commits: commits,
		}
	})
}
