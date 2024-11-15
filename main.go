package main

import (
	"flag"
	"fmt"
	"github.com/magiconair/properties"
	"log/slog"
	"pace/overlayNetwork"
	"pace/utils"
)

var logger = utils.GetLogger(slog.LevelWarn)

func main() {
	propsPathname := flag.String("config", "config/config.properties", "pathname of the configuration file")
	address := flag.String("address", "localhost:6000", "address of the current node")
	flag.Parse()
	props := properties.MustLoadFile(*propsPathname, properties.UTF8)
	contact := props.MustGetString("contact")
	node, err := overlayNetwork.NewNode(*address, contact)
	if err != nil {
		panic(fmt.Errorf("unable to create node: %v", err))
	}
	dealCode := props.MustGetString("deal_code")[0]
	dealSS := overlayNetwork.NewSSChannel(node, dealCode)

	numNodes := props.MustGetUint("num_nodes")

}
