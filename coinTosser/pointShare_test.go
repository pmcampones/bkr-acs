package coinTosser

import (
	"crypto/rand"
	"github.com/cloudflare/circl/group"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldUnmarshalBinaryPointShare(t *testing.T) {
	ogPt := pointShare{
		id:    group.Ristretto255.RandomScalar(rand.Reader),
		point: group.Ristretto255.RandomElement(rand.Reader),
	}
	data, err := ogPt.marshalBinary()
	assert.NoError(t, err)
	recovPt := emptyPointShare()
	err = recovPt.unmarshalBinary(data)
	assert.NoError(t, err)
	assert.Equal(t, ogPt.id, recovPt.id)
	assert.True(t, areElementsEqualsTest(t, ogPt.point, recovPt.point))
}
