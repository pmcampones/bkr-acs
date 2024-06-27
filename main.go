package main

import "C"
import (
	"broadcast_channels/broadcast"
	"broadcast_channels/network"
	"bufio"
	"github.com/lmittmann/tint"
	"log/slog"
	"os"
	"time"
)

type ConcreteObserver struct {
}

func (co ConcreteObserver) BCBDeliver(msg []byte) {
	println("BCB Deliver:", string(msg))
}

func (co ConcreteObserver) BEBDeliver(msg []byte) {
	println("BEB Deliver:", string(msg))
}

func main() {
	setupLogger()
	node := network.Join(os.Args[1], os.Args[2], os.Args[3], os.Args[4])
	testBCB(node)
	//testBEB(node)
}

func setupLogger() {
	// set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	))
	slog.Info("Set up logger")
}

func testBCB(node *network.Node) {
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

func testBEB(node *network.Node) {
	observer := ConcreteObserver{}
	node.AddObserver(observer)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		node.Broadcast([]byte(input.Text()))
	}
}
