package main

import (
	"broadcast_channels/broadcast"
	"broadcast_channels/network"
	"bufio"
	"os"
)

type ConcreteObserver struct {
}

func (co ConcreteObserver) BCBDeliver(msg []byte) {
	println(string(msg))
}

func main() {
	node := network.Join(os.Args[1], os.Args[2], os.Args[3], os.Args[4])
	observer := ConcreteObserver{}
	bcbChannel := broadcast.BCBCreateChannel(node, 4, 1)
	bcbChannel.AttachObserver(observer)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		err := bcbChannel.BCBroadcast([]byte(input.Text()))
		if err != nil {
			return
		}
	}
}
