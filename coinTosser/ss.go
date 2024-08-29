package coinTosser

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
const pointSize = 32
const shareSize = scalarSize * 2
const pointShareSize = scalarSize + pointSize

// PointShare is a secret share hidden in a group operation.
// This is used in the coin tossing scheme to hide the secret while making it usable as a randomness source.
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
		point: mulPoint(base, share.Value),
	}
}

func RecoverSecretFromPoints(shares []PointShare) group.Element {
	indices := lo.Map(shares, func(share PointShare, _ int) group.Scalar { return share.id })
	coefficients := lo.Map(indices, func(i group.Scalar, _ int) group.Scalar { return lagrangeCoefficient(i, indices) })
	terms := lo.ZipBy2(shares, coefficients, func(share PointShare, coeff group.Scalar) group.Element {
		return mulPoint(share.point, coeff)
	})
	return lo.Reduce(terms[1:], func(acc group.Element, term group.Element, _ int) group.Element {
		return addPoint(acc, term)
	}, terms[0])
}

func lagrangeCoefficient(i group.Scalar, indices []group.Scalar) group.Scalar {
	filteredIndices := lo.Filter(indices, func(j group.Scalar, _ int) bool { return !i.IsEqual(j) })
	numerators := lo.Reduce(filteredIndices, func(acc group.Scalar, j group.Scalar, _ int) group.Scalar {
		return mulScalar(neg(j), acc)
	}, newScalar(uint64(1)))
	denominators := lo.Reduce(filteredIndices, func(acc group.Scalar, j group.Scalar, _ int) group.Scalar {
		return mulScalar(acc, sub(i, j))
	}, newScalar(uint64(1)))
	return mulScalar(numerators, inv(denominators))
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
	if len(shareBytes) != shareSize {
		return secretsharing.Share{}, fmt.Errorf("argument has incorrect size: got %d bytes, expected %d", len(shareBytes), shareSize)
	}
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

func marshalPointShare(share PointShare) ([]byte, error) {
	idBytes, err := share.id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal share ID: %v", err)
	}
	pointBytes, err := share.point.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal share value: %v", err)
	}
	return append(idBytes, pointBytes...), nil
}

func unmarshalPointShare(shareBytes []byte) (PointShare, error) {
	if len(shareBytes) != pointShareSize {
		return PointShare{}, fmt.Errorf("argument has incorrect size: got %d bytes, expected %d", len(shareBytes), pointShareSize)
	}
	idBuffer := make([]byte, scalarSize)
	reader := bytes.NewReader(shareBytes)
	num, err := reader.Read(idBuffer)
	if err != nil {
		return PointShare{}, fmt.Errorf("unable to read share ID bytes: %v", err)
	} else if num != scalarSize {
		return PointShare{}, fmt.Errorf("unable to read share ID bytes: read %d bytes, expected %d", num, scalarSize)
	}
	id := group.Ristretto255.NewScalar()
	if err := id.UnmarshalBinary(idBuffer); err != nil {
		return PointShare{}, fmt.Errorf("unable to unmarshal share ID: %v", err)
	}
	pointBuffer := make([]byte, pointSize)
	num, err = reader.Read(pointBuffer)
	if err != nil {
		return PointShare{}, fmt.Errorf("unable to read share Value bytes: %v", err)
	} else if num != pointSize {
		return PointShare{}, fmt.Errorf("unable to read share Value bytes: read %d bytes, expected %d", num, pointSize)
	}
	point := group.Ristretto255.NewElement()
	if err := point.UnmarshalBinary(pointBuffer); err != nil {
		return PointShare{}, fmt.Errorf("unable to unmarshal share value: %v", err)
	}
	return PointShare{id: id, point: point}, nil
}
