package broadcast

import "fullMembership/network"

type bcastType byte

const (
	bcbMsg bcastType = 'A' + iota
	brbMsg
)

func BEB(n *network.Node, msg []byte) {
	n.Broadcast(msg)
}

func BCB(n *network.Node, msg []byte) {
	n.Broadcast(msg)
}

func BRB(n *network.Node, msg []byte) {
	n.Broadcast(msg)
}

func sliceJoin(slices ...[]byte) []byte {
	length := 0
	for _, aSlice := range slices {
		length += len(aSlice)
	}
	res := make([]byte, 0, length)
	for _, aSlice := range slices {
		res = append(res, aSlice...)
	}
	return res
}
