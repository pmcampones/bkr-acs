package coinTosser

import (
	"crypto/rand"
	"github.com/cloudflare/circl/group"
)

func mulScalar(a, b group.Scalar) group.Scalar {
	return group.Ristretto255.NewScalar().Mul(a, b)
}

func NewScalar(n uint64) group.Scalar {
	return group.Ristretto255.NewScalar().SetUint64(n)
}

func RandomScalar() group.Scalar {
	return group.Ristretto255.RandomScalar(rand.Reader)
}

func sub(a, b group.Scalar) group.Scalar {
	return group.Ristretto255.NewScalar().Sub(a, b)
}

func neg(a group.Scalar) group.Scalar {
	return group.Ristretto255.NewScalar().Neg(a)
}

func inv(a group.Scalar) group.Scalar {
	return group.Ristretto255.NewScalar().Inv(a)
}

func mulPoint(a group.Element, b group.Scalar) group.Element {
	return group.Ristretto255.NewElement().Mul(a, b)
}

func addPoint(a, b group.Element) group.Element {
	return group.Ristretto255.NewElement().Add(a, b)
}
