package main

import (
	"errors"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/log"
	"github.com/getsentry/sentry-go"

	// Register MySQL database driver.
	_ "github.com/go-sql-driver/mysql"
	"github.com/urfave/cli"
)

var version string

func init() {
	log.DefaultHandler.SetLevel(log.DEBUG)
	if version == "" {
		version = "v0.0.0"
	}
}

func main() {
	cfg := NewConfig()
	chunkSize := ChunkSize(1 * M)

	app := cli.NewApp()
	app.Version = version
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
		err := cfg.ReadFile(c.GlobalString("config"))
		if err != nil {
			log.Fatalln("Cannot read config:", err)
		}
		if c.IsSet("debug") {
			cfg.Debug = true
		}
		if c.IsSet("no-debug") {
			cfg.Debug = false
		}

		err = sentry.Init(sentry.ClientOptions{
			Dsn:     cfg.SentryDSN,
			Release: version,
		})
		if err != nil {
			log.Warningln("Cannot set Sentry DSN:", err)
		}
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "tracker",
			Usage: "runs Tracker process",
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
			Usage: "runs Server process",
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
				if path == "-" {
					return client.WriteReader(key, os.Stdin)
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
			Name:      "exists",
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
				exists, err := client.Exists(key)
				if err != nil {
					return err
				}
				if !exists {
					return errors.New("key does not exist")
				}
				return nil
			},
		},
		{
			Name:  "status",
			Usage: "show system status",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "sort, s",
					Usage: "sort devices by column",
					Value: "zone",
				},
			},
			Action: func(c *cli.Context) error {
				client, err := NewClient(cfg)
				if err != nil {
					return err
				}
				s, err := client.Status(c.String("sort"))
				if err != nil {
					return err
				}
				s.Print()
				return nil
			},
		},
		{
			Name:  "drain",
			Usage: "drain device by moving files to another device",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "dest, d",
					Usage: "move files to given devices",
				},
			},
			Action: func(c *cli.Context) error {
				d, err := NewDrainer(cfg)
				if err != nil {
					return err
				}
				destFlag := c.String("dest")
				if destFlag != "" {
					for _, devidString := range strings.Split(destFlag, ",") {
						devid, err := strconv.ParseInt(strings.TrimSpace(devidString), 10, 64)
						if err != nil {
							return err
						}
						d.Dest = append(d.Dest, devid)
					}
				}
				runUntilInterrupt(d)
				return nil
			},
		},
		{
			Name:   "ready",
			Hidden: true,
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "timeout",
					Value: 30 * time.Second,
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
		{
			Name: "mount",
			Action: func(c *cli.Context) error {
				mountPoint := c.Args().Get(0)
				return mount(cfg, mountPoint)
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
	err := p.Run()
	if err == nil {
		log.Notice("Process end successfully.")
		return
	}
	log.Fatal(err)
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
