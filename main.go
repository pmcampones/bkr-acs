package main

import (
	"flag"
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
	_, _ = overlayNetwork.NewNode(*address, contact)
}
