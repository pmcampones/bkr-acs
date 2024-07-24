package secretSharing

import (
	"fmt"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/group/edwards25519"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/xof/blake2xb"
)

// Dealer is a struct that represents a dealer in a VSS scheme.
// This is a simplification of the Dealer in the Kyber library.
// In Kyber, all dealings are encrypted and authenticated.
// In our case, communications are done over TLS, simplifying the Dealer struct.
type Dealer struct {
	secret     kyber.Scalar
	commits    []kyber.Point
	secretPoly *share.PriPoly
	verifiers  []kyber.Point
}

// Should not need this if the pull request to Kyber is merged
func ToScalar(val int64) kyber.Scalar {
	suite := edwards25519.NewBlakeSHA256Ed25519()
	return suite.Scalar().SetInt64(val)
}

func ShareSecret(t, n int, secret kyber.Scalar) []*share.PriShare {
	rng := blake2xb.New(nil)
	suite := edwards25519.NewBlakeSHA256Ed25519WithRand(rng)
	f := share.NewPriPoly(suite, t, secret, suite.RandomStream())
	shares := make([]*share.PriShare, n)
	for i := 0; i < n; i++ {
		shares[i] = f.Eval(i + 1)
	}
	return shares
}

func RecoverSecret(shares []*share.PriShare, t, n int) (kyber.Scalar, error) {
	suite := edwards25519.NewBlakeSHA256Ed25519()
	secret, err := share.RecoverSecret(suite, shares, t, n)
	if err != nil {
		return nil, fmt.Errorf("unable to recover secret: %v", err)
	}
	return secret, nil
}
