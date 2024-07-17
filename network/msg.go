package network

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
)

type msgType byte

const (
	membership msgType = 'A' + iota
	generic
)

func send(conn net.Conn, msg []byte) error {
	writer := bufio.NewWriter(conn)
	err := binary.Write(writer, binary.LittleEndian, uint32(len(msg)))
	if err != nil {
		return fmt.Errorf("unable to write message length to buffer: %v", err)
	}
	_, err = writer.Write(msg)
	if err != nil {
		return fmt.Errorf("unable to write message to buffer: %v", err)
	}
	// Don't defer Flush() to ensure the error message is returned if it fails
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("unable to flush buffer: %v", err)
	}
	return nil
}

func receive(conn net.Conn) ([]byte, error) {
	var length uint32
	reader := bufio.NewReader(conn)
	err := binary.Read(reader, binary.LittleEndian, &length)
	if err != nil {
		return nil, fmt.Errorf("unable to read message length from buffer: %v", err)
	}
	msg := make([]byte, length)
	num, err := reader.Read(msg)
	if err != nil {
		return nil, fmt.Errorf("unable to read message from buffer: %v", err)
	} else if num != int(length) {
		return nil, fmt.Errorf("unable to read message from buffer: read %d bytes, expected %d", num, length)
	}
	return msg, err
}
