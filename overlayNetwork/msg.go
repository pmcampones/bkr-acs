package overlayNetwork

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"unsafe"
)

type msgType byte

const (
	membership msgType = 'A' + iota
	generic
)

func send(conn net.Conn, msg []byte) error {
	writer := bufio.NewWriterSize(conn, len(msg)+int(unsafe.Sizeof(len(msg))))
	err := binary.Write(writer, binary.LittleEndian, uint32(len(msg)))
	if err != nil {
		return fmt.Errorf("unable to write message length to buffer: %v", err)
	}
	num, err := writer.Write(msg)
	if err != nil {
		return fmt.Errorf("unable to write message to buffer: %v", err)
	} else if num != len(msg) {
		return fmt.Errorf("unable to write message to buffer: wrote %d bytes, expected %d", num, len(msg))
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("unable to flush buffer: %v", err)
	}
	return nil
}

func receive(conn net.Conn) ([]byte, error) {
	var length uint32
	err := binary.Read(conn, binary.LittleEndian, &length)
	if err != nil {
		return nil, fmt.Errorf("unable to read message length from buffer: %v", err)
	}
	msg := make([]byte, length)
	_, err = io.ReadFull(conn, msg)
	return msg, err
}
