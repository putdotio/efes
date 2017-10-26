package command

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

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
	log.Println("waiting for shutdown call to return")
	<-shutdown
	log.Println("shutdown returned")
}

func handleSignals(p process, shutdown chan struct{}) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	first := true
	for sig := range signals {
		log.Println("got signal:", sig)
		if first {
			log.Print("shutting down process, will exit all running tasks are finished")
			log.Print("press Ctrl-C again to kill 'em all")
			go shutdownProcess(p, shutdown)
			first = false
		} else {
			log.Print("exiting with code 1")
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
