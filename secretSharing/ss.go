package secretSharing

import (
	"fmt"
	"github.com/samber/lo"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/group/edwards25519"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/xof/blake2xb"
)

var suite = edwards25519.NewBlakeSHA256Ed25519WithRand(blake2xb.New(nil))

type PointShare struct {
	idx   int
	point kyber.Point
}

// Should not need this if the pull request to Kyber is merged
func ToScalar(val int64) kyber.Scalar {
	return suite.Scalar().SetInt64(val)
}

func ShareSecret(t, n int, secret kyber.Scalar) []*share.PriShare {
	f := share.NewPriPoly(suite, t, secret, suite.RandomStream())
	shares := make([]*share.PriShare, n)
	for i := 0; i < n; i++ {
		shares[i] = f.Eval(i + 1)
	}
	return shares
}

func RecoverSecret(shares []*share.PriShare, t, n int) (kyber.Scalar, error) {
	secret, err := share.RecoverSecret(suite, shares, t, n)
	if err != nil {
		return nil, fmt.Errorf("unable to recover secret: %v", err)
	}
	return secret, nil
}

func ShareToPoint(shares *share.PriShare, base kyber.Point) PointShare {
	return PointShare{shares.I, base.Mul(shares.V, base)}
}

func RecoverSecretFromPoints(hiddenShares []PointShare) kyber.Point {
	indices := make([]int, len(hiddenShares))
	for i, point := range hiddenShares {
		indices[i] = point.idx
	}
	lagrangeCoefficients := lo.Map(indices, func(i, _ int) kyber.Scalar { return lagrangeCoefficient(i, indices) })
	terms := lo.ZipBy2(hiddenShares, lagrangeCoefficients, func(share PointShare, coeff kyber.Scalar) kyber.Point { return share.point.Mul(coeff, share.point) })
	res := terms[0]
	for i := 1; i < len(terms); i++ {
		res = res.Add(res, terms[i])
	}
	return res
}

func lagrangeCoefficient(i int, indices []int) kyber.Scalar {
	filteredIndices := lo.Filter(indices, func(j int, _ int) bool { return i != j })
	numerators := lo.Reduce(filteredIndices, func(acc, j, _ int) int { return acc * -j }, 1)
	denominators := lo.Reduce(filteredIndices, func(acc, j, _ int) int { return acc * (i - j) }, 1)
	scalarNum := ToScalar(int64(numerators))
	scalarDenom := ToScalar(int64(denominators))
	return scalarNum.Div(scalarNum, scalarDenom)
}
