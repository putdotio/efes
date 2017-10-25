package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/cenkalti/log"
	"github.com/codegangsta/cli"
	"github.com/putdotio/efes/command"
)

const version = "0.0.0"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	app := cli.NewApp()
	app.Version = version
	app.Usage = "Simple yet powerful distributed file system"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "/etc/efes.toml",
			Usage: "configuration file path",
		},
	}
	app.Commands = []cli.Command{
		command.NewTracker(),
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
