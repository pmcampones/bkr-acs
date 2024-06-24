package network

import (
	"crypto/tls"
	"net"
)

type peer struct {
	conn net.Conn
	id   string
}

func newOutbound(myId, address string, config *tls.Config) (peer, error) {
	conn, err := tls.Dial("tcp", address, config)
	if err != nil {
		return peer{}, err
	}
	err = send(conn, []byte(myId))
	if err != nil {
		return peer{}, err
	}
	peer := peer{
		conn: conn,
		id:   address,
	}
	return peer, nil
}

func getInbound(listener net.Listener) (peer, error) {
	conn, err := listener.Accept()
	if err != nil {
		return peer{}, err
	}
	id, err := receive(conn)
	if err != nil {
		return peer{}, err
	}
	peer := peer{
		conn: conn,
		id:   string(id),
	}
	return peer, nil
}
