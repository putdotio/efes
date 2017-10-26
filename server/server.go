package server

import (
	"database/sql"

	// Register MySQL database driver.
	_ "github.com/go-sql-driver/mysql"

	"github.com/cenkalti/log"
	"github.com/putdotio/efes/config"
)

// Server runs on storage servers.
type Server struct {
	config *config.Config
	db     *sql.DB
	log    log.Logger
}

// New returns a new Server instance.
func New(c *config.Config) (*Server, error) {
	s := &Server{
		config: c,
		log:    log.NewLogger("server"),
	}
	return s, nil
}

// Run this server in a blocking manner. Running server can be stopped with Shutdown().
func (s *Server) Run() error {
	if s.config.Server.Debug {
		s.log.SetLevel(log.DEBUG)
	}
	var err error
	s.db, err = sql.Open("mysql", s.config.Database.DSN)
	if err != nil {
		return err
	}
	// TODO implement
	s.log.Notice("Server is started.")
	return nil
}

// Shutdown the server.
func (s *Server) Shutdown() error {
	err := s.db.Close()
	if err != nil {
		s.log.Error("Error while closing database connection")
		return err
	}
	return nil
}
