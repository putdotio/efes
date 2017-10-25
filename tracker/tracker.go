package tracker

import (
	"database/sql"
	"net"
	"net/http"

	// Register MySQL database driver.
	_ "github.com/go-sql-driver/mysql"

	"github.com/putdotio/efes/config"
	"github.com/putdotio/efes/logger"
)

// Tracker tracks the info of files in database.
// Tracker responds to client requests.
// Tracker sends jobs to servers.
type Tracker struct {
	config   *config.TrackerConfig
	db       *sql.DB
	log      *logger.Logger
	listener net.Listener
	server   http.Server
	mux      *http.ServeMux
}

// New returns a new Tracker instance.
func New(c *config.TrackerConfig) (*Tracker, error) {
	t := &Tracker{
		config: c,
		log:    logger.New("tracker"),
	}
	if c.Debug {
		t.log.EnableDebug()
	}
	var err error
	t.db, err = sql.Open("mysql", c.DBDSN)
	if err != nil {
		return nil, err
	}
	t.listener, err = net.Listen("tcp", c.ListenAddress)
	if err != nil {
		return nil, err
	}
	t.mux = http.NewServeMux()
	t.mux.HandleFunc("/ping", t.ping)
	t.server.Handler = t.mux
	return t, nil
}

// Run this tracker in a blocking manner. Running tracker can be stopped with Close().
func (t *Tracker) Run() error {
	defer func() {
		if err := t.db.Close(); err != nil {
			t.log.Error("Error while closing database connection")
		}
	}()
	err := t.server.Serve(t.listener)
	t.log.Debug("Tracker is closed.")
	return err
}

// Close the tracker.
func (t *Tracker) Close() {
	// TODO
}

func (t *Tracker) ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}
