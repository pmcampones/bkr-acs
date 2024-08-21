package secretSharing

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/secretsharing"
	"log/slog"
	"net"
	"pace/network"
)

type Deal struct {
	share      *secretsharing.Share
	commitBase *group.Element
	commit     *group.Element
}

type DealObserver struct {
	Code         byte
	DealChan     chan *Deal
	hasBeenDealt bool
}

func (do *DealObserver) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	slog.Info("received message", "sender", sender, "Code", msg[0])
	if msg[0] == do.Code {
		if do.hasBeenDealt {
			slog.Error("deal already received")
		}
		share, commit := unmarshalDeal(msg)
		do.genDeal(&share, &commit)
	} else {
		slog.Debug("received message was not for me", "sender", sender, "Code", msg[0])
	}
}

func unmarshalDeal(msg []byte) (secretsharing.Share, group.Element) {
	shareBytes := make([]byte, shareSize)
	copy(shareBytes, msg[1:shareSize+1])
	commitBytes := make([]byte, pointSize)
	copy(commitBytes, msg[shareSize+1:])
	share, err := unmarshalShare(shareBytes)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal share: %v", err))
	}
	commit := group.Ristretto255.NewElement()
	err = commit.UnmarshalBinary(commitBytes)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal commit: %v", err))
	}
	return share, commit
}

func (do *DealObserver) genDeal(share *secretsharing.Share, commitBase *group.Element) {
	commit := group.Ristretto255.NewElement().Mul(*commitBase, share.Value)
	deal := &Deal{
		share:      share,
		commitBase: commitBase,
		commit:     &commit,
	}
	do.hasBeenDealt = true
	do.DealChan <- deal
}

func ShareDeals(threshold uint, node *network.Node, peers []net.Conn, dealCode byte, obs *DealObserver) error {
	g := group.Ristretto255
	secret := g.RandomScalar(rand.Reader)
	commitBase := g.RandomElement(rand.Reader)
	marshaledBase, err := commitBase.MarshalBinary()
	if err != nil {
		return fmt.Errorf("unable to marshal commit base: %v", err)
	}
	ss := ShareSecret(threshold, uint(len(peers))+1, secret)
	go obs.genDeal(&ss[0], &commitBase)
	for i, peer := range peers {
		err = shareDeal(node, peer, ss[i+1], marshaledBase, dealCode)
		if err != nil {
			return fmt.Errorf("unable to share deal to peer %d: %v", i, err)
		}
	}
	return nil
}

func shareDeal(node *network.Node, peer net.Conn, share secretsharing.Share, marshaledBase []byte, dealCode byte) error {
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
