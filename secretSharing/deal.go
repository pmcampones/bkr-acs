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
	share  *secretsharing.Share
	commit *group.Element
}

type dealObserver struct {
	code     byte
	dealChan chan *Deal
}

func (do *dealObserver) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	if msg[0] == do.code {
		share, commit := unmarshalDeal(msg)
		do.genDeal(&share, &commit)
	} else {
		slog.Debug("received message was not for me", "sender", sender, "code", msg[0])
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

func (do *dealObserver) genDeal(share *secretsharing.Share, commitBase *group.Element) {
	commit := group.Ristretto255.NewElement().Mul(*commitBase, share.Value)
	deal := &Deal{
		share:  share,
		commit: &commit,
	}
	do.dealChan <- deal
}

func shareDeals(threshold uint, node *network.Node, peers []net.Conn, dealCode byte, obs *dealObserver) error {
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
		return fmt.Errorf("unable to write deal code: %v", err)
	}
	marshaledShare, err := marshalShare(share)
	if err != nil {
		return fmt.Errorf("unable to marshal share: %v", err)
	}
	_, err = writer.Write(marshaledShare)
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
	node.Unicast(buf.Bytes(), peer)
	return nil
}
