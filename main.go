package main

import (
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/log"
	"github.com/urfave/cli"

	"github.com/putdotio/efes/config"
	"github.com/putdotio/efes/server"
	"github.com/putdotio/efes/tracker"
)

const version = "0.0.0"

func init() {
	rand.Seed(time.Now().UnixNano())
	log.DefaultHandler.SetLevel(log.DEBUG)
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
		{
			Name:  "tracker",
			Usage: "Runs Tracker process",
			Action: func(c *cli.Context) {
				cfg, err := config.New(c.GlobalString("config"))
				if err != nil {
					log.Fatal("Error while loading configuration. ", err)
				}
				t, err := tracker.New(cfg)
				if err != nil {
					log.Fatal("Error while initializing tracker. ", err)
				}
				runUntilInterrupt(t)
			},
		},
		{
			Name:  "server",
			Usage: "Runs Server process",
			Action: func(c *cli.Context) {
				cfg, err := config.New(c.GlobalString("config"))
				if err != nil {
					log.Fatal("Error while loading configuration. ", err)
				}
				dir := c.Args().Get(0)
				s, err := server.New(cfg, dir)
				if err != nil {
					log.Fatal("Error while initializing tracker. ", err)
				}
				runUntilInterrupt(s)
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type process interface {
	Run() error
	Shutdown() error
}

// runUntilInterrupt runs s until it exits cleanly.
// If SIGINT or SIGTERM is received, s is asked to shutdown gracefully.
// if SIGINT or SIGTERM is received again while waiting for shutdown, process exits with exit code 1.
func runUntilInterrupt(p process) {
	shutdown := make(chan struct{})
	go handleSignals(p, shutdown)
	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
	// Run returned with no error. Wait process.Shutdown to return before exit.
	<-shutdown
}

func handleSignals(p process, shutdown chan struct{}) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	first := true
	for sig := range signals {
		log.Notice("Got signal: ", sig)
		if first {
			log.Notice("Shutting down process. Process will exit when active requests are finished.")
			log.Notice("Press Ctrl-C again to kill the process.")
			go shutdownProcess(p, shutdown)
			first = false
		} else {
			log.Error("Exiting with code 1.")
			os.Exit(1)
		}
	}
}

func shutdownProcess(p process, shutdown chan struct{}) {
	if err := p.Shutdown(); err != nil {
		log.Fatal(err)
	}
	close(shutdown)
}
