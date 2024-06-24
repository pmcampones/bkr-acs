package network

import (
	"bufio"
	"encoding/binary"
	"net"
)

type messageType byte

const (
	membership messageType = 'A' + iota
	generic
)

func intToBytes(i uint32) []byte {
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, i)
	return bs
}

func bytesToInt(bs []byte) uint32 {
	return binary.LittleEndian.Uint32(bs)
}

func send(conn net.Conn, msg []byte) error {
	writer := bufio.NewWriter(conn)
	err := binary.Write(writer, binary.LittleEndian, uint32(len(msg)))
	if err != nil {
		return err
	}
	_, err = writer.Write(msg)
	if err != nil {
		return err
	}
	// Don't defer Flush() to ensure the error message is returned if it fails
	return writer.Flush()
}

func receive(conn net.Conn) ([]byte, error) {
	var length uint32
	reader := bufio.NewReader(conn)
	err := binary.Read(reader, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}
	msg := make([]byte, length)
	_, err = reader.Read(msg)
	return msg, err
}
