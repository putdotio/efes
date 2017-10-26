package command

import (
	"log"

	"github.com/putdotio/efes/config"
	"github.com/putdotio/efes/tracker"
	"github.com/urfave/cli"
)

// NewTracker returns a new cli.Command for running Efes Tracker process.
func NewTracker() cli.Command {
	return cli.Command{
		Name:  "tracker",
		Usage: "Runs Tracker process",
		Action: func(c *cli.Context) {
			cfg, err := config.New(c.GlobalString("config"))
			if err != nil {
				log.Fatal("Error while loading configuration. ", err)
			}
			t, err := tracker.New(&cfg.Tracker)
			if err != nil {
				log.Fatal("Error while initializing tracker. ", err)
			}
			runUntilInterrupt(t)
		},
	}
}
