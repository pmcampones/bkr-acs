package overlayNetwork

import (
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"net"
	"pace/utils"
)

var peerLogger = utils.GetLogger("Peer", slog.LevelWarn)

type listenerCloseError struct {
	err error
}

func (l listenerCloseError) Error() string {
	return fmt.Sprintf("listener closed:%v", l.err)
}

type peer struct {
	conn net.Conn
	name string
	pk   *ecdsa.PublicKey
	pkId uuid.UUID
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
	pkId, err := utils.PkToUUID(pk)
	if err != nil {
		return peer{}, fmt.Errorf("unable to convert public key to UUID: %v", err)
	}
	peer := peer{
		conn: conn,
		name: address,
		pk:   pk,
		pkId: pkId,
	}
	peerLogger.Debug("new outbound peer created", "address", address, "id", pkId)
	return peer, nil
}

func getInbound(listener net.Listener) (peer, error) {
	conn, err := listener.Accept()
	if err != nil {
		return peer{}, listenerCloseError{err: err}
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
	pkId, err := utils.PkToUUID(pk)
	if err != nil {
		return peer{}, fmt.Errorf("unable to convert public key to UUID: %v", err)
	}
	peer := peer{
		conn: conn,
		name: name,
		pk:   pk,
		pkId: pkId,
	}
	peerLogger.Debug("new inbound peer created", "address", name, "id", pkId)
	return peer, nil
}

func (p *peer) String() string {
	return fmt.Sprintf("%s:%v", p.name, p.pkId)
}
