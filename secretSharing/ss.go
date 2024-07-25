package secretSharing

import (
	"crypto/rand"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"github.com/samber/lo"
)

type PointShare struct {
	id    group.Scalar
	point group.Element
}

func ShareSecret(threshold uint, nodes uint, secret group.Scalar) []secretsharing.Share {
	secretSharing := secretsharing.New(rand.Reader, threshold, secret)
	return secretSharing.Share(nodes)
}

func RecoverSecret(threshold uint, shares []secretsharing.Share) (group.Scalar, error) {
	return secretsharing.Recover(threshold, shares)
}

func ShareToPoint(share secretsharing.Share, base group.Element) PointShare {
	return PointShare{
		id:    share.ID,
		point: base.Mul(base, share.Value),
	}
}

func RecoverSecretFromPoints(shares []PointShare) group.Element {
	indices := lo.Map(shares, func(share PointShare, _ int) group.Scalar { return share.id })
	coefficients := lo.Map(indices, func(i group.Scalar, _ int) group.Scalar { return lagrangeCoefficient(i, indices) })
	terms := lo.ZipBy2(shares, coefficients, func(share PointShare, coeff group.Scalar) group.Element {
		return share.point.Mul(share.point, coeff)
	})
	res := terms[0]
	for i := 1; i < len(terms); i++ {
		res = res.Add(res, terms[i])
	}
	return res
}

func lagrangeCoefficient(i group.Scalar, indices []group.Scalar) group.Scalar {
	filteredIndices := lo.Filter(indices, func(j group.Scalar, _ int) bool { return !i.IsEqual(j) })
	numerators := lo.Reduce(filteredIndices, func(acc group.Scalar, j group.Scalar, _ int) group.Scalar {
		return acc.Mul(j.Neg(j), acc)
	}, group.Ristretto255.NewScalar().SetUint64(uint64(1)))
	denominators := lo.Reduce(filteredIndices, func(acc group.Scalar, j group.Scalar, _ int) group.Scalar {
		return acc.Mul(acc, i.Sub(i, j))
	}, group.Ristretto255.NewScalar().SetUint64(uint64(1)))
	return numerators.Mul(numerators, denominators.Inv(denominators))
}
