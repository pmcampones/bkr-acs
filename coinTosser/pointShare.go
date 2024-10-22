package coinTosser

import (
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
)

// pointShare is a secret share hidden in a group operation.
// This is used in the coin tossing scheme to hide the secret while making it usable as a randomness source.
type pointShare struct {
	id    group.Scalar
	point group.Element
}

func shareToPoint(share secretsharing.Share, base group.Element) pointShare {
	return pointShare{
		id:    share.ID,
		point: mulPoint(base, share.Value),
	}
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
