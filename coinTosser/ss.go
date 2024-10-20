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

// pointShare is a secret share hidden in a group operation.
// This is used in the coin tossing scheme to hide the secret while making it usable as a randomness source.
type pointShare struct {
	id    group.Scalar
	point group.Element
}

func emptyPointShare() pointShare {
	return pointShare{
		id:    group.Ristretto255.NewScalar(),
		point: group.Ristretto255.NewElement(),
	}
}

func (ps *pointShare) marshalBinary() ([]byte, error) {
	idBytes, err := ps.id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal share ID: %v", err)
	}
	pointBytes, err := ps.point.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal share value: %v", err)
	}
	return append(idBytes, pointBytes...), nil
}

func (ps *pointShare) unmarshalBinary(data []byte) error {
	idSize, err := getScalarSize()
	if err != nil {
		return fmt.Errorf("unable to get scalar size: %v", err)
	}
	pointSize, err := getElementSize()
	if err != nil {
		return fmt.Errorf("unable to get element size: %v", err)
	} else if len(data) != idSize+pointSize {
		return fmt.Errorf("argument has incorrect size: got %d bytes, expected %d", len(data), idSize+pointSize)
	} else if err := ps.id.UnmarshalBinary(data[:idSize]); err != nil {
		return fmt.Errorf("unable to unmarshal share ID: %v", err)
	} else if err := ps.point.UnmarshalBinary(data[idSize:]); err != nil {
		return fmt.Errorf("unable to unmarshal share value: %v", err)
	}
	return nil
}

func getScalarSize() (int, error) {
	scalar := group.Ristretto255.NewScalar()
	scalarBytes, err := scalar.MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("unable to marshal scalar: %v", err)
	}
	return len(scalarBytes), nil
}

func getElementSize() (int, error) {
	element := group.Ristretto255.NewElement()
	elementBytes, err := element.MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("unable to marshal element: %v", err)
	}
	return len(elementBytes), nil
}

func getPointShareSize() (int, error) {
	scalarSize, err := getScalarSize()
	if err != nil {
		return 0, fmt.Errorf("unable to get scalar size: %v", err)
	}
	elementSize, err := getElementSize()
	if err != nil {
		return 0, fmt.Errorf("unable to get element size: %v", err)
	}
	return scalarSize + elementSize, nil
}

func ShareSecret(threshold uint, nodes uint, secret group.Scalar) []secretsharing.Share {
	secretSharing := secretsharing.New(rand.Reader, threshold, secret)
	return secretSharing.Share(nodes)
}

func RecoverSecret(threshold uint, shares []secretsharing.Share) (group.Scalar, error) {
	return secretsharing.Recover(threshold, shares)
}

func ShareToPoint(share secretsharing.Share, base group.Element) pointShare {
	return pointShare{
		id:    share.ID,
		point: mulPoint(base, share.Value),
	}
}

func RecoverSecretFromPoints(shares []pointShare) group.Element {
	indices := lo.Map(shares, func(share pointShare, _ int) group.Scalar { return share.id })
	coefficients := lo.Map(indices, func(i group.Scalar, _ int) group.Scalar { return lagrangeCoefficient(i, indices) })
	terms := lo.ZipBy2(shares, coefficients, func(share pointShare, coeff group.Scalar) group.Element {
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

func marshalPointShare(share pointShare) ([]byte, error) {
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

func unmarshalPointShare(shareBytes []byte) (pointShare, error) {
	if len(shareBytes) != pointShareSize {
		return pointShare{}, fmt.Errorf("argument has incorrect size: got %d bytes, expected %d", len(shareBytes), pointShareSize)
	}
	idBuffer := make([]byte, scalarSize)
	reader := bytes.NewReader(shareBytes)
	num, err := reader.Read(idBuffer)
	if err != nil {
		return pointShare{}, fmt.Errorf("unable to read share ID bytes: %v", err)
	} else if num != scalarSize {
		return pointShare{}, fmt.Errorf("unable to read share ID bytes: read %d bytes, expected %d", num, scalarSize)
	}
	id := group.Ristretto255.NewScalar()
	if err := id.UnmarshalBinary(idBuffer); err != nil {
		return pointShare{}, fmt.Errorf("unable to unmarshal share ID: %v", err)
	}
	pointBuffer := make([]byte, pointSize)
	num, err = reader.Read(pointBuffer)
	if err != nil {
		return pointShare{}, fmt.Errorf("unable to read share Value bytes: %v", err)
	} else if num != pointSize {
		return pointShare{}, fmt.Errorf("unable to read share Value bytes: read %d bytes, expected %d", num, pointSize)
	}
	point := group.Ristretto255.NewElement()
	if err := point.UnmarshalBinary(pointBuffer); err != nil {
		return pointShare{}, fmt.Errorf("unable to unmarshal share value: %v", err)
	}
	return pointShare{id: id, point: point}, nil
}
