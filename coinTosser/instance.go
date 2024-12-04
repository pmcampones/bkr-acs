package coinTosser

import (
	"bkr-acs/utils"
	"crypto"
	"encoding/binary"
	"fmt"
	"github.com/cloudflare/circl/group"
	"github.com/cloudflare/circl/zk/dleq"
	. "github.com/google/uuid"
	"log/slog"
	"math/rand"
	"unsafe"
)

var ctLogger = utils.GetLogger("CT Instance", slog.LevelWarn)

const dleqDst = "DLEQ"

type coinToss struct {
	base group.Element
	d    *deal
	shareProcessor
}

func newCoinToss(threshold uint, base group.Element, d *deal, outputChan chan bool) *coinToss {
	ct := &coinToss{
		base:           base,
		d:              d,
		shareProcessor: newShareProcessor(threshold, outputChan),
	}
	ctLogger.Info("new coin toss created", "threshold", threshold, "base", base)
	return ct
}

// Go does not allow structs as const values, so this is here to replace it :(
func getDLEQParams() dleq.Params {
	return dleq.Params{G: group.Ristretto255, H: crypto.SHA256, DST: []byte(dleqDst)}
}

func (ct *coinToss) tossCoin() (ctShare, error) {
	ctLogger.Info("tossing coin")
	share := shareToPoint(ct.d.share, ct.base)
	proof, err := ct.genProof(share.point)
	if err != nil {
		return ctShare{}, fmt.Errorf("unable to generate proof: %v", err)
	}
	return ctShare{pt: share, proof: proof}, nil
}

// Strange that the prover can choose the randomness seed for the proof
// TODO: I have to check the proof and see if this is secure. And if not warn the Circl devs
func (ct *coinToss) genProof(valToProve group.Element) (dleq.Proof, error) {
	seed := ct.computeProofSeed()
	myCommit, err := ct.d.getCommit(ct.d.share.ID)
	if err != nil {
		return dleq.Proof{}, fmt.Errorf("unable to get my commitment: %v", err)
	}
	params := getDLEQParams()
	prover := dleq.Prover{Params: params}
	proof, err := prover.ProveWithRandomness(ct.d.share.Value, ct.base, valToProve, ct.d.base, *myCommit, seed)
	if err != nil {
		return dleq.Proof{}, fmt.Errorf("unable to generate proof: %v", err)
	}
	return *proof, nil
}

func (ct *coinToss) computeProofSeed() group.Scalar {
	seed := make([]byte, unsafe.Sizeof(uint32(0)))
	binary.LittleEndian.PutUint32(seed, rand.Uint32())
	scalarSeed := group.Ristretto255.HashToScalar(seed, []byte("dleqSeed"))
	return scalarSeed
}

func (d *deal) getCommit(idx group.Scalar) (*group.Element, error) {
	for i, commit := range d.commits {
		ok, err := areScalarEquals(commit.id, idx)
		if err != nil {
			return nil, fmt.Errorf("unable to compare scalars of %d point share: %v", i, err)
		} else if ok {
			return &commit.point, nil
		}
	}
	return nil, fmt.Errorf("commitment not found for share %v", idx)
}

func (ct *coinToss) submitShare(ctShare ctShare, senderId UUID) error {
	ctLogger.Debug("received share", "share", ctShare.pt, "sender", senderId)
	isValid, err := ct.isTossValid(ctShare) // <-- Bottleneck
	if err != nil {
		return fmt.Errorf("unable to validate share from peer %v: %v", senderId, err)
	} else if !isValid {
		return fmt.Errorf("invalid share from peer %v", senderId)
	} else {
		return ct.processShare(ctShare.pt, senderId)
	}
}

func (ct *coinToss) isTossValid(share ctShare) (bool, error) {
	peerCommit, err := ct.d.getCommit(share.pt.id)
	if err != nil {
		return false, fmt.Errorf("unable to get peer commitment: %v", err)
	}
	verifier := dleq.Verifier{Params: getDLEQParams()}
	return verifier.Verify(ct.base, share.pt.point, ct.d.base, *peerCommit, &share.proof), nil
}

type shareProcessor struct {
	t            uint
	shares       []pointShare
	receivedFrom map[UUID]bool
	outputChan   chan bool
	commands     chan<- func()
	closeChan    chan struct{}
}

func newShareProcessor(t uint, outputChan chan bool) shareProcessor {
	commands := make(chan func())
	sp := shareProcessor{
		t:            t,
		shares:       make([]pointShare, 0),
		receivedFrom: make(map[UUID]bool),
		outputChan:   outputChan,
		commands:     commands,
		closeChan:    make(chan struct{}),
	}
	go sp.invoker(commands, sp.closeChan)
	return sp
}

func (sp *shareProcessor) processShare(share pointShare, senderId UUID) error {
	errChan := make(chan error)
	sp.commands <- func() {
		if sp.receivedFrom[senderId] {
			errChan <- fmt.Errorf("peer %v already sent share", senderId)
		}
		sp.receivedFrom[senderId] = true
		sp.shares = append(sp.shares, share)
		ctLogger.Debug("received share", "num received", len(sp.shares), "required", sp.t+1)
		if len(sp.shares) == int(sp.t+1) {
			ctLogger.Info("received all shares required to compute coin")
			secretPoint := recoverSecretFromPoints(sp.shares)
			coin, err := hashPointToBool(secretPoint)
			ctLogger.Info("computed random coin", "coin", coin)
			if err != nil {
				errChan <- fmt.Errorf("unable to hash point to bool: %v", err)
			}
			go func() { sp.outputChan <- coin }()
		}
		errChan <- nil
	}
	return <-errChan
}

func (sp *shareProcessor) invoker(commands <-chan func(), closeChan <-chan struct{}) {
	for {
		select {
		case command := <-commands:
			command()
		case <-closeChan:
			ctLogger.Info("closing share processor")
			return
		}
	}
}

func (sp *shareProcessor) close() {
	ctLogger.Info("sending signal to close share processor")
	sp.closeChan <- struct{}{}
}

func (ct *coinToss) close() {
	ct.close()
}
