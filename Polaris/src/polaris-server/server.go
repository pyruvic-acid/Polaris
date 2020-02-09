package main

import (
	"Polaris/src/common"
	"Polaris/src/server"
	"flag"

	logging "github.com/op/go-logging"
	"math/rand"
	"time"
)

var logger = logging.MustGetLogger("server.main")

var serverId string = ""
var configHome = "config"
var isDebug = true

func main() {
	parseArgs()
	rand.Seed(time.Now().UnixNano())
	common.ConfigLogger(isDebug)
	configPath := configHome + "/server_" + serverId + ".json"

	logger.Infof("server %v start", serverId)
	server := server.NewServer(configPath, configHome)

	server.Start()
}

func parseArgs() {
	flag.StringVar(
		&serverId,
		"id",
		"",
		"server id",
	)
	flag.BoolVar(
		&isDebug,
		"debug",
		false,
		"true to enable debug mode",
	)

	flag.StringVar(
		&configHome,
		"config",
		"",
		"config home",
	)

	flag.Parse()

	if serverId == "" || configHome == "" {
		logger.Fatal("Invalid configuration file or server id.")
		flag.Usage()
	}
}
