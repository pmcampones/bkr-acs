package network

import (
	"broadcast_channels/crypto"
	"bytes"
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"unsafe"
)

type peer struct {
	conn net.Conn
	name string
	pk   *ecdsa.PublicKey
}

func newOutbound(myName, address string, config *tls.Config, sk *ecdsa.PrivateKey) (peer, error) {
	conn, err := tls.Dial("tcp", address, config)
	if err != nil {
		return peer{}, fmt.Errorf("unable to dial while establishing peer connection: %s", err)
	}
	toSend, err := computeInitializationInfo(myName, err, sk)
	if err != nil {
		return peer{}, fmt.Errorf("unable to prepare the information to exchange with peer: %s", err)
	}
	err = send(conn, toSend)
	if err != nil {
		return peer{}, fmt.Errorf("unable to exchange initialization information to peer: %s", err)
	}
	peer := peer{
		conn: conn,
		name: address,
		pk:   &sk.PublicKey,
	}
	return peer, nil
}

func computeInitializationInfo(myName string, err error, sk *ecdsa.PrivateKey) ([]byte, error) {
	nonce := rand.Uint32()
	if err != nil {
		return nil, fmt.Errorf("unable to serialize my public key to send to peer:%s", err)
	}
	// 4 bytes for the length of the name, the name, 4 bytes for the nonce, the public key, and 64 bytes for the signature
	data, err := computeSignableData(myName, nonce, &sk.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("unable to compute signable data to send to peer: %s", err)
	}
	signature, err := crypto.Sign(sk, data)
	if err != nil {
		return nil, fmt.Errorf("unable to sign initialization information to peer: %s", err)
	}
	data = append(data, signature...)
	return data, nil
}

func getInbound(listener net.Listener) (peer, error) {
	conn, err := listener.Accept()
	if err != nil {
		return peer{}, fmt.Errorf("unable to accept inbount connection with peer: %s", err)
	}
	data, err := receive(conn)
	if err != nil {
		return peer{}, fmt.Errorf("unable to receive initialization information of peer: %s", err)
	}
	name, data, err := readName(data)
	if err != nil {
		return peer{}, fmt.Errorf("unable to read name of peer: %s", err)
	}
	nonce, data, err := readNonce(data)
	if err != nil {
		return peer{}, fmt.Errorf("unable to read nonce of peer: %s", err)
	}
	pk, data, err := readPk(data)
	if err != nil {
		return peer{}, fmt.Errorf("unable to read public key of peer: %s", err)
	}
	signature, err := computeSignableData(name, nonce, pk)
	if err != nil {
		return peer{}, fmt.Errorf("unable to compute signable data of peer: %s", err)
	}
	if !crypto.Verify(pk, signature, data) {
		return peer{}, fmt.Errorf("peer signature is incorrect")
	}
	peer := peer{
		conn: conn,
		name: name,
		pk:   pk,
	}
	return peer, nil
}

func computeSignableData(name string, nonce uint32, pk *ecdsa.PublicKey) ([]byte, error) {
	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)
	buf.Write(crypto.IntToBytes(uint32(len(name))))
	buf.Write([]byte(name))
	buf.Write(crypto.IntToBytes(nonce))
	pkBytes, err := crypto.SerializePublicKey(pk)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize public key of peer: %s", err)
	}
	buf.Write(crypto.IntToBytes(uint32(len(pkBytes))))
	buf.Write(pkBytes)
	return buf.Bytes(), nil
}

func readName(bytes []byte) (string, []byte, error) {
	lenSize := uint32(unsafe.Sizeof(uint32(0)))
	if lenSize > uint32(len(bytes)) {
		return "", bytes, fmt.Errorf("not enough bytes to read name length")
	}
	nameLen, err := crypto.BytesToInt(bytes[:lenSize])
	if err != nil {
		return "", bytes, fmt.Errorf("unable to read name length of peer: %s", err)
	}
	if lenSize+nameLen > uint32(len(bytes)) {
		return "", bytes, fmt.Errorf("not enough bytes to read name")
	}
	name := string(bytes[lenSize : lenSize+nameLen])
	return name, bytes[lenSize+nameLen:], nil
}

func readNonce(bytes []byte) (uint32, []byte, error) {
	lenSize := int(unsafe.Sizeof(uint32(0)))
	if lenSize > len(bytes) {
		return 0, bytes, fmt.Errorf("not enough bytes to read nonce")
	}
	nonce, err := crypto.BytesToInt(bytes[:lenSize])
	if err != nil {
		return 0, bytes, fmt.Errorf("unable to read nonce of peer: %s", err)
	}
	return nonce, bytes[lenSize:], nil
}

func readPk(bytes []byte) (*ecdsa.PublicKey, []byte, error) {
	lenSize := uint32(unsafe.Sizeof(uint32(0)))
	if lenSize > uint32(len(bytes)) {
		return nil, bytes, fmt.Errorf("not enough bytes to read public key length")
	}
	pkLen, err := crypto.BytesToInt(bytes[:lenSize])
	if err != nil {
		return nil, bytes, fmt.Errorf("unable to read public key length of peer: %s", err)
	}
	if lenSize+pkLen > uint32(len(bytes)) {
		return nil, bytes, fmt.Errorf("not enough bytes to read public key")
	}
	pk, err := crypto.DeserializePublicKey(bytes[lenSize : lenSize+pkLen])
	if err != nil {
		return nil, bytes, fmt.Errorf("unable to deserialize public key of peer: %s", err)
	}
	return pk, bytes[lenSize+pkLen:], nil
}
