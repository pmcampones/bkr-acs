package network

import (
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
	"net"
)

type peer struct {
	conn net.Conn
	name string
	pk   *ecdsa.PublicKey
}

func newOutbound(myName, address string, config *tls.Config) (peer, error) {
	conn, err := tls.Dial("tcp", address, config)
	if err != nil {
		return peer{}, fmt.Errorf("unable to dial while establishing peer connection: %v", err)
	}
	err = send(conn, []byte(myName))
	if err != nil {
		return peer{}, fmt.Errorf("unable to send name of peer: %v", err)
	}
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return peer{}, fmt.Errorf("no certificates found in connection")
	}
	pk := certs[0].PublicKey.(*ecdsa.PublicKey)
	peer := peer{
		conn: conn,
		name: address,
		pk:   pk,
	}
	return peer, nil
}

func getInbound(listener net.Listener) (peer, error) {
	conn, err := listener.Accept()
	if err != nil {
		return peer{}, fmt.Errorf("unable to accept inbount connection with peer: %s", err)
	}
	nameBytes, err := receive(conn)
	if err != nil {
		return peer{}, fmt.Errorf("unable to receive initialization information of peer: %s", err)
	}
	name := string(nameBytes)
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return peer{}, fmt.Errorf("no certificates found in connection")
	}
	pk := certs[0].PublicKey.(*ecdsa.PublicKey)
	peer := peer{
		conn: conn,
		name: name,
		pk:   pk,
	}
	return peer, nil
}
