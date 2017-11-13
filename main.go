package main

import (
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/log"
	"github.com/urfave/cli"
	// Register MySQL database driver.
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.DefaultHandler.SetLevel(log.DEBUG)

	var cfg *Config

	app := cli.NewApp()
	app.Usage = "Simple yet powerful distributed file system"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config, c",
			Value:  "/etc/efes.toml",
			Usage:  "configuration file path",
			EnvVar: "EFES_CONFIG",
		},
		cli.BoolFlag{
			Name:   "debug, d",
			Usage:  "enable debug log",
			EnvVar: "EFES_DEBUG",
		},
	}
	app.Before = func(c *cli.Context) error {
		var err error
		cfg, err = ReadConfig(c.GlobalString("config"))
		if err != nil {
			log.Warningln("Cannot read config:", err)
		}
		if c.IsSet("debug") {
			cfg.Debug = c.GlobalBool("debug")
		}
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "tracker",
			Usage: "Runs Tracker process",
			Action: func(c *cli.Context) error {
				t, err := NewTracker(cfg)
				if err != nil {
					return err
				}
				runUntilInterrupt(t)
				return nil
			},
		},
		{
			Name:  "server",
			Usage: "Runs Server process",
			Action: func(c *cli.Context) error {
				s, err := NewServer(cfg)
				if err != nil {
					return err
				}
				runUntilInterrupt(s)
				return nil
			},
		},
		{
			Name:  "client",
			Usage: "Client for reading/writing files",
			Subcommands: []cli.Command{
				{
					Name:  "write",
					Usage: "write file to efes",
					Action: func(c *cli.Context) error {
						key := c.Args().Get(0)
						path := c.Args().Get(1)
						client, err := NewClient(cfg)
						if err != nil {
							return err
						}
						return client.Write(key, path)
					},
				},
				{
					Name:  "read",
					Usage: "read file from efes",
					Action: func(c *cli.Context) error {
						key := c.Args().Get(0)
						path := c.Args().Get(1)
						client, err := NewClient(cfg)
						if err != nil {
							return err
						}
						return client.Read(key, path)
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type process interface {
	// Run does the main work for the process.
	// Run must return nil when Shutdown is called.
	Run() error
	// Shutdown stops the running Run method.
	// After shutdown is called Run must return.
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
	log.Notice("Process shut down successfully.")
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
