package main

import (
	"errors"
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

func init() {
	log.DefaultHandler.SetLevel(log.DEBUG)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var cfg *Config
	var chunkSize = ChunkSize(1 * M)

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
			Name:  "debug, d",
			Usage: "enable debug log",
		},
		cli.BoolFlag{
			Name:  "no-debug, D",
			Usage: "disable debug log",
		},
	}
	app.Before = func(c *cli.Context) error {
		var err error
		cfg, err = ReadConfig(c.GlobalString("config"))
		if err != nil {
			log.Warningln("Cannot read config:", err)
		}
		if c.IsSet("debug") {
			cfg.Debug = true
		}
		if c.IsSet("no-debug") {
			cfg.Debug = false
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
					Name:      "write",
					Usage:     "write file to efes",
					ArgsUsage: "key path",
					Flags: []cli.Flag{
						cli.GenericFlag{
							Name:  "chunk, c",
							Usage: "chunk size",
							Value: &chunkSize,
						},
					},
					Action: func(c *cli.Context) error {
						if c.NArg() < 2 {
							cli.ShowAppHelpAndExit(c, 1)
						}
						key := c.Args().Get(0)
						path := c.Args().Get(1)
						if c.IsSet("chunk") {
							cfg.Client.ChunkSize = chunkSize
						}
						client, err := NewClient(cfg)
						if err != nil {
							return err
						}
						return client.Write(key, path)
					},
				},
				{
					Name:      "read",
					Usage:     "read file from efes",
					ArgsUsage: "key path",
					Action: func(c *cli.Context) error {
						if c.NArg() < 2 {
							cli.ShowAppHelpAndExit(c, 1)
						}
						key := c.Args().Get(0)
						path := c.Args().Get(1)
						client, err := NewClient(cfg)
						if err != nil {
							return err
						}
						return client.Read(key, path)
					},
				},
				{
					Name:      "delete",
					Usage:     "delete file from efes",
					ArgsUsage: "key",
					Action: func(c *cli.Context) error {
						if c.NArg() < 1 {
							cli.ShowAppHelpAndExit(c, 1)
						}
						key := c.Args().Get(0)
						client, err := NewClient(cfg)
						if err != nil {
							return err
						}
						return client.Delete(key)
					},
				},
				{
					Name:      "exist",
					Usage:     "check if a key exists in efes",
					ArgsUsage: "key",
					Action: func(c *cli.Context) error {
						if c.NArg() < 1 {
							cli.ShowAppHelpAndExit(c, 1)
						}
						key := c.Args().Get(0)
						client, err := NewClient(cfg)
						if err != nil {
							return err
						}
						exist, err := client.Exist(key)
						if err != nil {
							return err
						}
						if !exist {
							return errors.New("key does not exist")
						}
						return nil
					},
				},
				{
					Name:  "status",
					Usage: "show system status",
					Action: func(c *cli.Context) error {
						client, err := NewClient(cfg)
						if err != nil {
							return err
						}
						s, err := client.Status()
						if err != nil {
							return err
						}
						s.Print()
						return nil
					},
				},
			},
		},
		{
			Name:   "ready",
			Hidden: true,
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "timeout",
					Value: 10 * time.Second,
				},
			},
			Subcommands: []cli.Command{
				{
					Name: "mysql",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name: "exec",
						},
					},
					Action: func(c *cli.Context) error {
						return readyMysql(cfg.Database, c.Parent().Duration("timeout"), c.String("exec"))
					},
				},
				{
					Name: "rabbitmq",
					Action: func(c *cli.Context) error {
						return readyRabbitmq(cfg.AMQP, c.Parent().Duration("timeout"))
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
