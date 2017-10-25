package command

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

type process interface {
	Run() error
	Close()
}

// runUntilInterrupt runs s until it exits cleanly.
// If SIGINT or SIGTERM is received, s is asked to shutdown gracefully.
// if SIGINT or SIGTERM is received again while waiting for shutdown, process exits with exit code 1.
func runUntilInterrupt(s process) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		first := true
		for sig := range c {
			log.Println("got signal:", sig)
			if first {
				log.Print("shutting down process, will exit all running tasks are finished")
				log.Print("press Ctrl-C again to kill 'em all")
				s.Close()
				first = false
			} else {
				log.Print("exiting with code 1")
				os.Exit(1)
			}
		}
	}()

	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}
