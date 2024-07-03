package network

import (
	"broadcast_channels/crypto"
	"bytes"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
)

type peer struct {
	conn net.Conn
	name string
	pk   *ecdsa.PublicKey
}

func newOutbound(myName, address string, config *tls.Config, sk *ecdsa.PrivateKey) (peer, error) {
	conn, err := tls.Dial("tcp", address, config)
	if err != nil {
		return peer{}, fmt.Errorf("unable to dial while establishing peer connection: %v", err)
	}
	toSend, err := computeInitializationInfo(myName, err, sk)
	if err != nil {
		return peer{}, fmt.Errorf("unable to prepare the information to exchange with peer: %v", err)
	}
	err = send(conn, toSend)
	if err != nil {
		return peer{}, fmt.Errorf("unable to exchange initialization information to peer: %v", err)
	}
	pkBytes, err := receive(conn)
	if err != nil {
		return peer{}, fmt.Errorf("unable to receive peer's public key: %v", err)
	}
	pk, err := crypto.DeserializePublicKey(pkBytes)
	if err != nil {
		return peer{}, fmt.Errorf("unable to deserialize peer's public key: %v", err)
	}
	peer := peer{
		conn: conn,
		name: address,
		pk:   pk,
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

func getInbound(listener net.Listener, myPk *ecdsa.PublicKey) (peer, error) {
	conn, err := listener.Accept()
	if err != nil {
		return peer{}, fmt.Errorf("unable to accept inbount connection with peer: %s", err)
	}
	data, err := receive(conn)
	if err != nil {
		return peer{}, fmt.Errorf("unable to receive initialization information of peer: %s", err)
	}
	reader := bytes.NewReader(data)
	name, nonce, pk, err := readSignatureElements(reader)
	if err != nil {
		return peer{}, fmt.Errorf("unable to read data elements of peer: %s", err)
	}
	sigData, err := computeSignableData(name, nonce, pk)
	if err != nil {
		return peer{}, fmt.Errorf("unable to compute signable data of peer: %s", err)
	}
	signature := make([]byte, reader.Len())
	num, err := reader.Read(signature)
	if err != nil {
		return peer{}, fmt.Errorf("unable to read signature of peer: %s", err)
	} else if num != len(signature) {
		return peer{}, fmt.Errorf("unable to read signature of peer: read %d bytes, expected %d", num, len(signature))
	}
	if !crypto.Verify(pk, sigData, signature) {
		return peer{}, fmt.Errorf("unable to verify signature of peer")
	}
	pkBytes, err := crypto.SerializePublicKey(myPk)
	if err != nil {
		return peer{}, fmt.Errorf("unable to serialize my public key to send the peer: %v", err)
	}
	err = send(conn, pkBytes)
	if err != nil {
		return peer{}, fmt.Errorf("unable to send peer my public key: %v", err)
	}
	peer := peer{
		conn: conn,
		name: name,
		pk:   pk,
	}
	return peer, nil
}

func readSignatureElements(reader *bytes.Reader) (string, uint32, *ecdsa.PublicKey, error) {
	name, err := readName(reader)
	if err != nil {
		return "", 0, nil, fmt.Errorf("unable to read name of peer: %s", err)
	}
	var nonce uint32
	err = binary.Read(reader, binary.LittleEndian, &nonce)
	if err != nil {
		return "", 0, nil, fmt.Errorf("unable to read nonce of peer: %s", err)
	}
	pk, err := readPk(reader)
	if err != nil {
		return "", 0, nil, fmt.Errorf("unable to read public key of peer: %s", err)
	}
	return name, nonce, pk, nil
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

func readName(reader *bytes.Reader) (string, error) {
	var nameLen uint32
	err := binary.Read(reader, binary.LittleEndian, &nameLen)
	if err != nil {
		return "", fmt.Errorf("unable to read name length of peer: %s", err)
	}
	nameBytes := make([]byte, nameLen)
	num, err := reader.Read(nameBytes)
	if err != nil {
		return "", fmt.Errorf("unable to read name of peer: %s", err)
	} else if num != int(nameLen) {
		return "", fmt.Errorf("unable to read name of peer: read %d bytes, expected %d", num, nameLen)
	}
	return string(nameBytes), nil
}

func readPk(reader *bytes.Reader) (*ecdsa.PublicKey, error) {
	var pkLen uint32
	err := binary.Read(reader, binary.LittleEndian, &pkLen)
	if err != nil {
		return nil, fmt.Errorf("unable to read public key length of peer: %s", err)
	}
	pkBytes := make([]byte, pkLen)
	num, err := reader.Read(pkBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to read public key of peer: %s", err)
	} else if num != int(pkLen) {
		return nil, fmt.Errorf("unable to read public key of peer: read %d bytes, expected %d", num, pkLen)
	}
	pk, err := crypto.DeserializePublicKey(pkBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to deserialize public key of peer: %s", err)
	}
	return pk, nil
}
