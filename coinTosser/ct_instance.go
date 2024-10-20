package coinTosser

import (
	"crypto"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/zk/dleq"
	. "github.com/google/uuid"
	"log/slog"
	"pace/utils"
)

var ctLogger = utils.GetLogger(slog.LevelWarn)

const dleqDst = "DLEQ"

type coinObserver interface {
	observeCoin(id UUID, toss bool)
}

type coinTossShare struct {
	ptShare pointShare
	proof   dleq.Proof
}

type coinToss struct {
	id            UUID
	threshold     uint
	base          group.Element
	deal          Deal
	hiddenShares  []pointShare
	observers     []coinObserver
	peersReceived map[UUID]bool
	commands      chan<- func() error
	closeChan     chan struct{}
}

func newCoinToss(id UUID, threshold uint, base group.Element, deal Deal) *coinToss {
	commands := make(chan func() error)
	ct := &coinToss{
		id:            id,
		threshold:     threshold,
		base:          base,
		deal:          deal,
		hiddenShares:  make([]pointShare, 0),
		observers:     make([]coinObserver, 0),
		peersReceived: make(map[UUID]bool),
		commands:      commands,
		closeChan:     make(chan struct{}),
	}
	go ct.invoker(commands, ct.closeChan)
	return ct
}

func (ct *coinToss) AttachObserver(observer coinObserver) {
	ct.observers = append(ct.observers, observer)
}

// Go does not allow structs as const values, so this is here to replace it :(
func getDLEQParams() dleq.Params {
	return dleq.Params{G: group.Ristretto255, H: crypto.SHA256, DST: []byte(dleqDst)}
}

func (ct *coinToss) tossCoin() (coinTossShare, error) {
	share := ShareToPoint(ct.deal.share, ct.base)
	proof, err := ct.genProof(share.point)
	if err != nil {
		return coinTossShare{}, fmt.Errorf("unable to generate proof: %v", err)
	}
	return coinTossShare{ptShare: share, proof: proof}, nil
}

func (ct *coinToss) genProof(valToProve group.Element) (dleq.Proof, error) {
	idBytes, err := ct.id.MarshalBinary()
	if err != nil {
		return dleq.Proof{}, fmt.Errorf("unable to marshal id: %v", err)
	}
	params := getDLEQParams()
	seed := group.Ristretto255.HashToScalar(idBytes, []byte("dleqSeed"))
	prover := dleq.Prover{Params: params}
	proof, err := prover.ProveWithRandomness(ct.deal.share.Value, ct.base, valToProve, ct.deal.commitBase, ct.deal.commit, seed)
	if err != nil {
		return dleq.Proof{}, fmt.Errorf("unable to generate proof: %v", err)
	}
	return *proof, nil
}

func (ct *coinToss) getShare(ctShare coinTossShare, senderId UUID) error {
	isValid, err := ct.isTossValid(ctShare, senderId)
	if err != nil {
		return fmt.Errorf("unable to validate share: %v", err)
	} else if !isValid {
		return fmt.Errorf("invalid share from peer %v", senderId)
	}
	ct.commands <- func() error {
		if ct.peersReceived[senderId] {
			return fmt.Errorf("peer %v already sent share", senderId)
		}
		ct.peersReceived[senderId] = true
		return ct.processShare(ctShare.ptShare)
	}
	return nil
}

func (ct *coinToss) isTossValid(ctShare coinTossShare, senderId UUID) (bool, error) {
	proof := ctShare.proof
	peerCommit, ok := ct.deal.peerCommits[senderId]
	if !ok {
		return false, fmt.Errorf("peer %v commitment not found", senderId)
	}
	verifier := dleq.Verifier{Params: getDLEQParams()}
	return verifier.Verify(ct.base, ctShare.ptShare.point, ct.deal.commitBase, peerCommit, &proof), nil
}

func (ct *coinToss) processShare(share pointShare) error {
	ct.hiddenShares = append(ct.hiddenShares, share)
	ctLogger.Debug("received share", "id", ct.id, "numShares", len(ct.hiddenShares))
	if len(ct.hiddenShares) == int(ct.threshold)+1 {
		secretPoint := RecoverSecretFromPoints(ct.hiddenShares)
		coin, err := HashPointToBool(secretPoint)
		if err != nil {
			return fmt.Errorf("unable to hash point to bool: %v", err)
		}
		ctLogger.Debug("recovered secret", "id", ct.id, "secret", secretPoint, "coin", coin)
		for _, observer := range ct.observers {
			go observer.observeCoin(ct.id, coin)
		}
	}
	return nil
}

func marshalCoinTossShare(share coinTossShare) ([]byte, error) {
	ptShareBytes, err := marshalPointShare(share.ptShare)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal point share: %v", err)
	}
	proofBytes, err := share.proof.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal proof: %v", err)
	}
	return append(ptShareBytes, proofBytes...), nil
}

func unmarshalCoinTossShare(shareBytes []byte) (coinTossShare, error) {
	ptShare, err := unmarshalPointShare(shareBytes[:pointShareSize])
	if err != nil {
		return coinTossShare{}, fmt.Errorf("unable to unmarshal point share: %v", err)
	}
	proof := dleq.Proof{}
	err = proof.UnmarshalBinary(getDLEQParams().G, shareBytes[pointShareSize:])
	if err != nil {
		return coinTossShare{}, fmt.Errorf("unable to unmarshal proof: %v", err)
	}
	return coinTossShare{ptShare: ptShare, proof: proof}, nil
}

func (ct *coinToss) invoker(commands <-chan func() error, closeChan <-chan struct{}) {
	for {
		select {
		case command := <-commands:
			err := command()
			if err != nil {
				ctLogger.Error("unable to compute command", "id", ct.id, "error", err)
			}
		case <-closeChan:
			ctLogger.Debug("closing brb executor", "id", ct.id)
			return
		}
	}
}

func (ct *coinToss) close() {
	ctLogger.Debug("sending signal to close bcb instance", "Id", ct.id)
	ct.closeChan <- struct{}{}
}
