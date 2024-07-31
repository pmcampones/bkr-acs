package secretSharing

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"github.com/samber/lo"
)

const scalarSize = 32

// PointShare is a secret share hidden in a group operation.
// This is used in the coin tossing scheme to hide the secret while making it usable as a randomness source.
type PointShare struct {
	id    group.Scalar
	point group.Element
}

// PointCommit is a secret commitment hidden in a group operation.
// This value is unique for each node and is used verify a PointShare with the same idx is correct using a DLEQ proof.
type PointCommit struct {
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

// HashPointToBool hashes a point to a boolean value
// Not sure if this is secure and unbiased.
func HashPointToBool(point group.Element) (bool, error) {
	pointMarshal, err := point.MarshalBinary()
	if err != nil {
		return false, fmt.Errorf("unable to generate bytes from point: %v", err)
	}
	hashed := sha256.Sum256(pointMarshal)
	sum := lo.Reduce(hashed[:], func(acc int, b byte, _ int) int { return acc + int(b) }, 0)
	return sum%2 == 0, nil
}

func marshalShare(share secretsharing.Share) ([]byte, error) {
	idBytes, err := share.ID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal share ID: %v", err)
	}
	valueBytes, err := share.Value.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal share value: %v", err)
	}
	return append(idBytes, valueBytes...), nil
}

func unmarshalShare(shareBytes []byte) (secretsharing.Share, error) {
	buffer := make([]byte, scalarSize)
	reader := bytes.NewReader(shareBytes)
	num, err := reader.Read(buffer)
	if err != nil {
		return secretsharing.Share{}, fmt.Errorf("unable to read share ID bytes: %v", err)
	} else if num != scalarSize {
		return secretsharing.Share{}, fmt.Errorf("unable to read share ID bytes: read %d bytes, expected %d", num, scalarSize)
	}
	id := group.Ristretto255.NewScalar()
	if err := id.UnmarshalBinary(buffer); err != nil {
		return secretsharing.Share{}, fmt.Errorf("unable to unmarshal share ID: %v", err)
	}
	num, err = reader.Read(buffer)
	if err != nil {
		return secretsharing.Share{}, fmt.Errorf("unable to read share Value bytes: %v", err)
	} else if num != scalarSize {
		return secretsharing.Share{}, fmt.Errorf("unable to read share Value bytes: read %d bytes, expected %d", num, scalarSize)
	}
	value := group.Ristretto255.NewScalar()
	if err := value.UnmarshalBinary(buffer); err != nil {
		return secretsharing.Share{}, fmt.Errorf("unable to unmarshal share value: %v", err)
	}
	return secretsharing.Share{ID: id, Value: value}, nil
}
