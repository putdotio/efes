package tracker

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	t.mux.HandleFunc("/get-paths", t.getPaths)
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

func (t *Tracker) getPaths(w http.ResponseWriter, r *http.Request) {
	var response struct {
		Paths []string `json:"paths"`
	}
	response.Paths = make([]string, 0)
	key := r.FormValue("key")
	rows, err := t.db.Query("select h.hostip, h.http_port, d.devid, f.fid from file f join file_on fo on f.fid=fo.fid join device d on d.devid=fo.devid join host h on h.hostid=d.hostid where f.dkey=?", key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var hostip string
		var httpPort int64
		var devid int64
		var fid int64
		err := rows.Scan(&hostip, &httpPort, &devid, &fid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		sfid := fmt.Sprintf("%010d", fid)
		path := fmt.Sprintf("http://%s:%d/dev%d/%s/%s/%s/%s.fid", hostip, httpPort, devid, sfid[0:1], sfid[1:4], sfid[4:7], sfid)
		response.Paths = append(response.Paths, path)
	}
	err = rows.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(response)
}
