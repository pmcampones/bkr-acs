package coinTosser

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"github.com/cloudflare/circl/group"
	. "github.com/google/uuid"
	"github.com/samber/mo"
	"io"
	"log/slog"
	"pace/overlayNetwork"
	"pace/utils"
)

var channelLogger = utils.GetLogger(slog.LevelDebug)

type CoinObserver interface {
	DeliverCoin(id UUID, toss bool)
}

type CTChannel struct {
	myself         UUID
	instances      map[UUID]*coinToss
	finished       map[UUID]bool
	outputChannels map[UUID]chan mo.Result[bool]
	unordered      map[UUID][]func() error
	t              uint
	deal           Deal
	dealObs        *DealObserver
	network        *overlayNetwork.Node
	commands       chan<- func() error
	listenCode     byte
}

func NewCoinTosserChannel(node *overlayNetwork.Node, t uint) *CTChannel {
	commands := make(chan func() error)
	myself, err := utils.PkToUUID(node.GetPk())
	if err != nil {
		panic(fmt.Errorf("unable to convert public key to UUID: %v", err))
	}
	channel := &CTChannel{
		myself:         myself,
		instances:      make(map[UUID]*coinToss),
		outputChannels: make(map[UUID]chan mo.Result[bool]),
		unordered:      make(map[UUID][]func() error),
		finished:       make(map[UUID]bool),
		t:              t,
		dealObs:        NewDealObserver(),
		network:        node,
		commands:       commands,
		listenCode:     utils.GetCode("ct_code"),
	}
	node.AttachMessageObserver(channel)
	node.AttachMessageObserver(channel.dealObs)
	go func() {
		deal := <-channel.dealObs.dealChan
		channel.deal = *deal
		channelLogger.Info("received deal. Starting to process commands")
		invoker(commands)
	}()
	return channel
}

func (c *CTChannel) ShareDeal() error {
	peers := c.network.GetPeers()
	channelLogger.Info("sharing deal", "myself", c.myself, "peers", peers)
	return shareDeals(c.t, c.network, peers, c.dealObs)
}

func (c *CTChannel) TossCoin(seed []byte, outputChan chan mo.Result[bool]) {
	id := utils.BytesToUUID(seed)
	channelLogger.Debug("issuing coin toss", "id", id, "myself", c.myself)
	c.commands <- func() error {
		base := group.Ristretto255.HashToElement(seed, []byte("coin_toss"))
		coinTosserInstance := newCoinToss(id, c.t, base, c.deal)
		c.instances[id] = coinTosserInstance
		coinTosserInstance.AttachObserver(c)
		c.outputChannels[id] = outputChan
		channelLogger.Debug("tossing coin", "id", id, "myself", c.myself)
		ctShare, err := coinTosserInstance.tossCoin()
		if err != nil {
			return fmt.Errorf("unable to create toss coin share: %v", err)
		}
		msg, err := c.makeCoinTossMessage(id, ctShare)
		err = c.network.Broadcast(msg)
		if err != nil {
			return fmt.Errorf("unable to broadcast coin toss message: %v", err)
		}
		c.processUnordered(id)
		return nil
	}
}

func (c *CTChannel) processUnordered(id UUID) {
	for _, command := range c.unordered[id] {
		err := command()
		if err != nil {
			outputChan := c.outputChannels[id]
			channelLogger.Warn("error processing unordered coin toss share", "id", id, "error", err, "myself", c.myself)
			go func() {
				outputChan <- mo.Err[bool](err)
			}()
		}
	}
	delete(c.unordered, id)
}

func (c *CTChannel) makeCoinTossMessage(id UUID, ctShare coinTossShare) ([]byte, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to marshal id: %v", err)
	}
	ctsBytes, err := marshalCoinTossShare(ctShare)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal coin toss share: %v", err)
	}
	buff := bytes.NewBuffer(make([]byte, 0, len(ctsBytes)+len(idBytes)+1))
	writer := bufio.NewWriter(buff)
	err = writer.WriteByte(c.listenCode)
	if err != nil {
		return nil, fmt.Errorf("unable to write listen code: %v", err)
	}
	_, err = writer.Write(idBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to write id: %v", err)
	}
	_, err = writer.Write(ctsBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to write coin toss share: %v", err)
	}
	err = writer.Flush()
	if err != nil {
		return nil, fmt.Errorf("unable to flush writer: %v", err)
	}
	return buff.Bytes(), nil
}

func readCoinTossMessage(msg []byte) (UUID, coinTossShare, error) {
	reader := bytes.NewReader(msg)
	id, err := utils.ExtractIdFromMessage(reader)
	if err != nil {
		return UUID{}, coinTossShare{}, fmt.Errorf("unable to extract id from message: %v", err)
	}
	coinTossBytes := make([]byte, reader.Len())
	_, err = io.ReadFull(reader, coinTossBytes)
	if err != nil {
		return UUID{}, coinTossShare{}, fmt.Errorf("unable to read coin toss share: %v", err)
	}
	ctShare, err := unmarshalCoinTossShare(coinTossBytes)
	if err != nil {
		return UUID{}, coinTossShare{}, fmt.Errorf("unable to unmarshal coin toss share: %v", err)
	}
	return id, ctShare, nil
}

func (c *CTChannel) BEBDeliver(msg []byte, sender *ecdsa.PublicKey) {
	if msg[0] == c.listenCode {
		msg = msg[1:]
		id, ctShare, err := readCoinTossMessage(msg)
		if err != nil {
			channelLogger.Error("unable to read coin toss message", "error", err)
			return
		}
		senderId, err := utils.PkToUUID(sender)
		if err != nil {
			channelLogger.Error("unable to convert public key to UUID", "error", err)
			return
		}
		command := func() error {
			channelLogger.Debug("submitting coin toss share", "id", id, "sender", senderId, "myself", c.myself)
			return c.submitShare(id, senderId, ctShare)
		}
		c.scheduleShareSubmission(id, senderId, command)
	}
}

func (c *CTChannel) submitShare(id, senderId UUID, ctShare coinTossShare) error {
	if c.finished[id] {
		return nil
	}
	ct := c.instances[id]
	if ct == nil {
		return fmt.Errorf("coin toss instance not found")
	}
	channelLogger.Debug("received coin toss share", "id", id, "sender", senderId, "myself", c.myself)
	err := ct.getShare(ctShare, senderId)
	if err != nil {
		return fmt.Errorf("unable to get share: %v", err)
	}
	return nil
}

func (c *CTChannel) scheduleShareSubmission(id UUID, senderId UUID, command func() error) {
	c.commands <- func() error {
		if c.instances[id] == nil {
			if c.unordered[id] == nil {
				c.unordered[id] = make([]func() error, 0)
			}
			channelLogger.Debug("received unordered coin toss share", "id", id, "sender", senderId, "myself", c.myself)
			c.unordered[id] = append(c.unordered[id], command)
		} else {
			return command()
		}
		return nil
	}
}

func (c *CTChannel) observeCoin(id UUID, toss bool) {
	channelLogger.Debug("issuing coin delivery", "id", id, "toss", toss, "myself", c.myself)
	c.commands <- func() error {
		channelLogger.Debug("delivering coin", "id", id, "toss", toss, "myself", c.myself)
		c.finished[id] = true
		delete(c.instances, id)
		outputChan := c.outputChannels[id]
		go func() {
			outputChan <- mo.Ok(toss)
		}()
		return nil
	}
}

func invoker(commands <-chan func() error) {
	for command := range commands {
		err := command()
		if err != nil {
			channelLogger.Error("error executing command", "error", err)
		}
	}
}
