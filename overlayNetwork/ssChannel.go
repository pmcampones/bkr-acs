package overlayNetwork

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"github.com/cloudflare/circl/group"
	ss "github.com/cloudflare/circl/secretsharing"
	"github.com/samber/lo"
	"log/slog"
	"pace/utils"
)

var ssLogger = utils.GetLogger("SSChannel", slog.LevelWarn)

type SSMsg struct {
	Share      ss.Share
	Commitment []byte
	Sender     *ecdsa.PublicKey
	Err        error
}

type SSChannel struct {
	node        *Node
	listenCode  byte
	deliverChan chan *SSMsg
}

func NewSSChannel(node *Node, listenCode byte) *SSChannel {
	s := &SSChannel{
		node:        node,
		listenCode:  listenCode,
		deliverChan: make(chan *SSMsg),
	}
	node.attachMessageObserver(s)
	ssLogger.Info("ss channel created", "listenCode", listenCode)
	return s
}

func (s *SSChannel) SSBroadcast(secret group.Scalar, threshold uint, commitMaker func([]ss.Share) ([]byte, error)) error {
	ssLogger.Debug("broadcasting secret shares", "secret", secret, "threshold", threshold)
	secretSharing := ss.New(rand.Reader, threshold, secret)
	peers := s.node.getPeers()
	shares := secretSharing.Share(uint(len(peers) + 1))
	shareMsgs := make([][]byte, len(shares))
	commitment, err := commitMaker(shares)
	if err != nil {
		return fmt.Errorf("unable to make commitment: %v", err)
	}
	for i, share := range shares {
		msg, err := s.wrapMsg(share, commitment)
		if err != nil {
			return fmt.Errorf("unable to wrap message: %v", err)
		}
		shareMsgs[i] = msg
	}
	if err := s.node.unicastSelf(shareMsgs[0]); err != nil {
		return fmt.Errorf("unable to unicast self: %v", err)
	}
	for _, tuple := range lo.Zip2(peers, shareMsgs[1:]) {
		peer, msg := tuple.Unpack()
		if err := s.node.unicast(msg, peer.conn); err != nil {
			nodeLogger.Warn("error sending ss to connection", "peer name", peer.name, "error", err)
		}
	}
	return nil
}

func (s *SSChannel) wrapMsg(share ss.Share, commitment []byte) ([]byte, error) {
	idBytes, err := share.ID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal ID: %w", err)
	}
	contentBytes, err := share.Value.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal value: %w", err)
	}
	buf := bytes.NewBuffer(make([]byte, 0, 1+len(idBytes)+len(contentBytes)+len(commitment)))
	buf.WriteByte(s.listenCode)
	buf.Write(idBytes)
	buf.Write(contentBytes)
	buf.Write(commitment)
	return buf.Bytes(), nil
}

func (s *SSChannel) bebDeliver(msg []byte, sender *ecdsa.PublicKey) {
	if msg[0] == s.listenCode {
		nodeLogger.Debug("bebDeliver in ss channel", "msg", msg, "sender", sender)
		msg = msg[1:]
		id := group.Ristretto255.NewScalar()
		val := group.Ristretto255.NewScalar()
		if scalarSize, err := utils.GetScalarSize(); err != nil {
			s.deliverChan <- &SSMsg{Err: fmt.Errorf("unable to get scalar size: %w", err)}
		} else if len(msg) < 2*scalarSize {
			s.deliverChan <- &SSMsg{Err: fmt.Errorf("message too short")}
		} else if err := id.UnmarshalBinary(msg[:scalarSize]); err != nil {
			s.deliverChan <- &SSMsg{Err: fmt.Errorf("unable to unmarshal ID: %w", err)}
		} else if err := val.UnmarshalBinary(msg[scalarSize : 2*scalarSize]); err != nil {
			s.deliverChan <- &SSMsg{Err: fmt.Errorf("unable to unmarshal value: %w", err)}
		} else {
			s.deliverChan <- &SSMsg{
				Share:      ss.Share{ID: id, Value: val},
				Commitment: msg[2*scalarSize:],
				Sender:     sender,
			}
		}
	}
}

func (s *SSChannel) GetSSChan() <-chan *SSMsg {
	return s.deliverChan
}
