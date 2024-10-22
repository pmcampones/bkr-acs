package coinTosser

import (
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/zk/dleq"
)

type ctShare struct {
	pt    pointShare
	proof dleq.Proof
}

func emptyCTShare() ctShare {
	return ctShare{
		pt:    emptyPointShare(),
		proof: dleq.Proof{},
	}
}

func (s *ctShare) unmarshalBinary(data []byte) error {
	pt := emptyPointShare()
	proof := dleq.Proof{}
	if ptSize, err := getPointShareSize(); err != nil {
		return fmt.Errorf("unable to get point share size: %v", err)
	} else if err := pt.unmarshalBinary(data[:ptSize]); err != nil {
		return fmt.Errorf("unable to unmarshal point share: %v", err)
	} else if err := proof.UnmarshalBinary(group.Ristretto255, data[ptSize:]); err != nil {
		return fmt.Errorf("unable to unmarshal proof: %v", err)
	}
	s.pt = pt
	s.proof = proof
	return nil
}

func (s *ctShare) marshalBinary() ([]byte, error) {
	ptBytes, err := s.pt.marshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal point share: %v", err)
	}
	proofBytes, err := s.proof.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal proof: %v", err)
	}
	return append(ptBytes, proofBytes...), nil
}
