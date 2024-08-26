package secretSharing

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	. "github.com/google/uuid"
	"github.com/samber/lo"
	"io"
	"log/slog"
	"net"
	"pace/network"
	"pace/utils"
)

var dLogger = utils.GetLogger(slog.LevelInfo)

type Deal struct {
	share       secretsharing.Share
	commitBase  group.Element
	commit      group.Element
	peerCommits map[UUID]group.Element
}

type DealObserver struct {
	Code         byte
	DealChan     chan *Deal
	hasBeenDealt bool
}

func (do *DealObserver) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	dLogger.Debug("received message", "sender", sender, "Code", msg[0])
	if msg[0] == do.Code {
		if do.hasBeenDealt {
			dLogger.Error("deal already received")
		}
		share, commitBase, commits := unmarshalDeal(msg[1:])
		do.genDeal(&share, commitBase, commits)
	} else {
		dLogger.Debug("received message was not for me", "sender", sender, "Code", msg[0])
	}
}

func unmarshalDeal(msg []byte) (secretsharing.Share, group.Element, []lo.Tuple2[group.Element, ecdsa.PublicKey]) {
	shareBytes := make([]byte, shareSize)
	copy(shareBytes, msg[:shareSize])
	commitBytes := make([]byte, pointSize)
	copy(commitBytes, msg[shareSize:shareSize+pointSize])
	share, err := unmarshalShare(shareBytes)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal share: %v", err))
	}
	commitBase := group.Ristretto255.NewElement()
	err = commitBase.UnmarshalBinary(commitBytes)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal commitBase: %v", err))
	}
	commits, err := deserializeCommitments(msg[shareSize+pointSize:])
	if err != nil {
		panic(fmt.Errorf("unable to deserialize commitments: %v", err))
	}
	return share, commitBase, commits
}

func (do *DealObserver) genDeal(share *secretsharing.Share, commitBase group.Element, commits []lo.Tuple2[group.Element, ecdsa.PublicKey]) {
	commit := mulPoint(commitBase, share.Value)
	deal := &Deal{
		share:       *share,
		commitBase:  commitBase,
		commit:      commit,
		peerCommits: make(map[UUID]group.Element, len(commits)),
	}
	for _, tuple := range commits {
		commit, pk := tuple.Unpack()
		pkBytes, _ := utils.SerializePublicKey(&pk)
		id := utils.BytesToUUID(pkBytes)
		deal.peerCommits[id] = commit
	}
	do.hasBeenDealt = true
	dLogger.Info("delivering deal")
	do.DealChan <- deal
}

func ShareDeals(threshold uint, node *network.Node, peers []*network.Peer, dealCode byte, obs *DealObserver) error {
	g := group.Ristretto255
	secret := g.RandomScalar(rand.Reader)
	commitBase := g.RandomElement(rand.Reader)
	marshaledBase, err := commitBase.MarshalBinary()
	if err != nil {
		return fmt.Errorf("unable to marshal commit base: %v", err)
	}
	ss := ShareSecret(threshold, uint(len(peers))+1, secret)
	pks := lo.Map(peers, func(peer *network.Peer, _ int) ecdsa.PublicKey {
		return *peer.Pk
	})
	pks = append([]ecdsa.PublicKey{*node.GetPk()}, pks...)
	commits, err := computeCommitments(ss, commitBase, pks)
	if err != nil {
		return fmt.Errorf("unable to compute commitments: %v", err)
	}
	serializedCommits, err := serializeCommitments(commits)
	if err != nil {
		return fmt.Errorf("unable to serialize commitments: %v", err)
	}
	go obs.genDeal(&ss[0], commitBase, commits)
	dLogger.Info("sending deal to peers", "peers", peers)
	for i, peer := range peers {
		err = shareDeal(node, peer.Conn, ss[i+1], marshaledBase, serializedCommits, dealCode)
		if err != nil {
			return fmt.Errorf("unable to share deal to peer %d: %v", i, err)
		}
	}
	return nil
}

func computeCommitments(ss []secretsharing.Share, commitBase group.Element, pks []ecdsa.PublicKey) ([]lo.Tuple2[group.Element, ecdsa.PublicKey], error) {
	if len(ss) != len(pks) {
		return nil, fmt.Errorf("number of shares and public keys must be the same")
	}
	peerCommitValues := lo.Map(ss, func(share secretsharing.Share, _ int) group.Element {
		return mulPoint(commitBase, share.Value)
	})
	return lo.Zip2(peerCommitValues, pks), nil
}

func serializeCommitments(commits []lo.Tuple2[group.Element, ecdsa.PublicKey]) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	err := writer.WriteByte(byte(len(commits)))
	if err != nil {
		return nil, fmt.Errorf("unable to write number of commitments to serialize: %v", err)
	}
	for i, tuple := range commits {
		val, pk := tuple.Unpack()
		valBytes, err := val.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to marshal %d-th commit point: %v", i, err)
		}
		_, err = writer.Write(valBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to write the %d-th commit point: %v", i, err)
		}
		pkBytes, err := utils.SerializePublicKey(&pk)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal %d-th public key: %v", i, err)
		}
		err = binary.Write(writer, binary.LittleEndian, uint16(len(pkBytes)))
		if err != nil {
			return nil, fmt.Errorf("unable to write the length of the %d-th serialized public key: %v", i, err)
		}
		_, err = writer.Write(pkBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to write the %d-th public key: %v", i, err)
		}
	}
	err = writer.Flush()
	if err != nil {
		return nil, fmt.Errorf("unable to flush writer: %v", err)
	}
	return buf.Bytes(), nil
}

func deserializeCommitments(data []byte) ([]lo.Tuple2[group.Element, ecdsa.PublicKey], error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	numCommits, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("unable to read how many commits were received: %v", err)
	}
	commits := make([]lo.Tuple2[group.Element, ecdsa.PublicKey], 0, numCommits)
	for i := 0; i < int(numCommits); i++ {
		pointBytes := make([]byte, pointSize)
		_, err := io.ReadFull(reader, pointBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to read %d-th commit point bytes: %v", i, err)
		}
		point := group.Ristretto255.NewElement()
		err = point.UnmarshalBinary(pointBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal %d-th commit point: %v", i, err)
		}
		var pkLen uint16
		err = binary.Read(reader, binary.LittleEndian, &pkLen)
		if err != nil {
			return nil, fmt.Errorf("unable to read %d-th commit number of bytes in pk: %v", i, err)
		}
		pkBytes := make([]byte, pkLen)
		_, err = io.ReadFull(reader, pkBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to read %d-th commit pk: %v", i, err)
		}
		pk, err := utils.DeserializePublicKey(pkBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to deserialize %d-th commit pk: %v ", i, err)
		}
		commits = append(commits, lo.Tuple2[group.Element, ecdsa.PublicKey]{A: point, B: *pk})
	}
	return commits, nil
}

func shareDeal(node *network.Node, peer net.Conn, share secretsharing.Share, marshaledBase, commits []byte, dealCode byte) error {
	buf := bytes.NewBuffer([]byte{})
	writer := bufio.NewWriter(buf)
	err := writer.WriteByte(dealCode)
	if err != nil {
		return fmt.Errorf("unable to write deal Code: %v", err)
	}
	marshaledShare, err := marshalShare(share)
	if err != nil {
		return fmt.Errorf("unable to marshal share: %v", err)
	}
	_, err = writer.Write(marshaledShare[:])
	if err != nil {
		return fmt.Errorf("unable to write share: %v", err)
	}
	_, err = writer.Write(marshaledBase)
	if err != nil {
		return fmt.Errorf("unable to write commit base: %v", err)
	}
	_, err = writer.Write(commits)
	if err != nil {
		return fmt.Errorf("unable to write commits: %v", err)
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("unable to flush writer: %v", err)
	}
	err = node.Unicast(buf.Bytes(), peer)
	if err != nil {
		return fmt.Errorf("unable to send deal to peer: %v", err)
	}
	return nil
}
