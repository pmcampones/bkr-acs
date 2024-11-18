package coinTosser

import (
	on "bkr-acs/overlayNetwork"
	"bkr-acs/utils"
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/cloudflare/circl/group"
	ss "github.com/cloudflare/circl/secretsharing"
	"github.com/samber/lo"
	"log/slog"
	"unsafe"
)

var dealLogger = utils.GetLogger("Deal", slog.LevelWarn)

func DealSecret(ssChannel *on.SSChannel, secret group.Scalar, threshold uint) error {
	dealLogger.Info("dealing secret", "secret", secret, "threshold", threshold)
	return ssChannel.SSBroadcast(secret, threshold, computeCommitment)
}

func computeCommitment(shares []ss.Share) ([]byte, error) {
	base := group.Ristretto255.RandomElement(rand.Reader)
	commits := lo.Map(shares, func(share ss.Share, _ int) pointShare { return shareToPoint(share, base) })
	return marshalCommitment(base, commits)
}

func marshalCommitment(base group.Element, commits []pointShare) ([]byte, error) {
	baseBytes, err := base.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal base: %v", err)
	}
	commitsBytes := make([][]byte, len(commits))
	for i, commit := range commits {
		commitBytes, err := commit.marshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to marshal commit: %v", err)
		}
		commitsBytes[i] = commitBytes
	}
	return writeCommitment(baseBytes, commitsBytes)
}

func writeCommitment(base []byte, commits [][]byte) ([]byte, error) {
	commitLen := len(commits)
	commitLenBytes := make([]byte, unsafe.Sizeof(uint32(commitLen)))
	binary.LittleEndian.PutUint32(commitLenBytes, uint32(commitLen))
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	if _, err := writer.Write(base); err != nil {
		return nil, fmt.Errorf("unable to write base: %v", err)
	} else if _, err := writer.Write(commitLenBytes); err != nil {
		return nil, fmt.Errorf("unable to write commitment length: %v", err)
	}
	for i, commit := range commits {
		if _, err := writer.Write(commit); err != nil {
			return nil, fmt.Errorf("unable to write %d-th commitment: %v", i, err)
		}
	}
	if err := writer.Flush(); err != nil {
		return nil, fmt.Errorf("unable to flush writer: %v", err)
	}
	return buf.Bytes(), nil
}

type deal struct {
	base    group.Element
	share   ss.Share
	commits []pointShare
}

func listenDeal(ssChan <-chan *on.SSMsg) (*deal, error) {
	ssMsg := <-ssChan
	if ssMsg.Err != nil {
		return nil, fmt.Errorf("received error message: %v", ssMsg.Err)
	}
	share := ssMsg.Share
	base, commits, err := unmarshalCommitment(ssMsg.Commitment)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal commitment: %v", err)
	}
	dealLogger.Info("received deal", "share", share, "base", base, "commits", commits)
	return &deal{base, share, commits}, nil
}

func unmarshalCommitment(data []byte) (group.Element, []pointShare, error) {
	base, data, err := unmarshalBase(data)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to unmarshal base: %v", err)
	}
	commitLen, data, err := unmarshalCommitLen(data)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to unmarshal commitment length: %v", err)
	}
	pointShares, err := unmarshalPointShares(data, int(commitLen))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to unmarshal point shares: %v", err)
	}
	return base, pointShares, nil
}

func unmarshalBase(dataIn []byte) (group.Element, []byte, error) {
	base := group.Ristretto255.NewElement()
	baseLen, err := getElementSize()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get element length: %v", err)
	} else if len(dataIn) < baseLen {
		return nil, nil, fmt.Errorf("argument is too short: got %d bytes, expected at least %d", len(dataIn), baseLen)
	} else if err := base.UnmarshalBinary(dataIn[:baseLen]); err != nil {
		return nil, nil, fmt.Errorf("unable to unmarshal base: %v", err)
	}
	return base, dataIn[baseLen:], nil
}

func unmarshalCommitLen(dataIn []byte) (uint32, []byte, error) {
	commitLenLen := unsafe.Sizeof(uint32(0))
	if len(dataIn) < int(commitLenLen) {
		return 0, nil, fmt.Errorf("argument is too short: got %d bytes, expected at least %d", len(dataIn), commitLenLen)
	}
	commitLen := binary.LittleEndian.Uint32(dataIn[:commitLenLen])
	return commitLen, dataIn[commitLenLen:], nil
}

func unmarshalPointShares(dataIn []byte, num int) ([]pointShare, error) {
	pointShareLen, err := getPointShareSize()
	if err != nil {
		return nil, fmt.Errorf("unable to get point share length: %v", err)
	} else if len(dataIn) != num*pointShareLen {
		return nil, fmt.Errorf("argument is too short: got %d bytes, expected %d", len(dataIn), num*pointShareLen)
	}
	pointShares := make([]pointShare, num)
	for i := 0; i < num; i++ {
		pointShare := emptyPointShare()
		if err := pointShare.unmarshalBinary(dataIn[:pointShareLen]); err != nil {
			return nil, fmt.Errorf("unable to unmarshal %d-th point share: %v", i, err)
		}
		pointShares[i] = pointShare
		dataIn = dataIn[pointShareLen:]
	}
	return pointShares, nil
}
