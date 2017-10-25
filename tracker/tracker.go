package tracker

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"

	"github.com/putdotio/efes/config"
	"github.com/putdotio/efes/logger"
)

// Tracker tracks the info of files in database.
// Tracker responds to client requests.
// Tracker sends jobs to servers.
type Tracker struct {
	config *config.TrackerConfig
	db     *sql.DB
	log    *logger.Logger
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
		return t, err
	}
	return t, nil
}

// Run this tracker in a blocking manner. Running tracker can be stopped with Close().
func (t *Tracker) Run() error {
	defer func() {
		if err := t.db.Close(); err != nil {
			t.log.Error("Error while closing database connection")
		}
	}()
	t.log.Debug("Tracker is closed.")
	return nil
}

func (t *Tracker) Close() {
	// TODO
}
