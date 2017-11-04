package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/log"
)

// TODO remove hardcoded constants
const (
	dmid    = 1
	classid = 1
)

// Tracker tracks the info of files in database.
// Tracker responds to client requests.
// Tracker sends jobs to servers.
type Tracker struct {
	config *Config
	db     *sql.DB
	log    log.Logger
	server http.Server
}

// NewTracker returns a new Tracker instance.
func NewTracker(c *Config) (*Tracker, error) {
	t := &Tracker{
		config: c,
		log:    log.NewLogger("tracker"),
	}
	m := http.NewServeMux()
	m.HandleFunc("/ping", t.ping)
	m.HandleFunc("/get-paths", t.getPaths)
	m.HandleFunc("/create-open", t.createOpen)
	m.HandleFunc("/create-close", t.createClose)
	m.HandleFunc("/delete", t.deleteFile)
	t.server.Handler = m
	if t.config.Tracker.Debug {
		t.log.SetLevel(log.DEBUG)
	}
	var err error
	t.db, err = sql.Open("mysql", t.config.Database.DSN)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Run this tracker in a blocking manner. Running tracker can be stopped with Shutdown().
func (t *Tracker) Run() error {
	listener, err := net.Listen("tcp", t.config.Tracker.ListenAddress)
	if err != nil {
		return err
	}
	t.log.Notice("Tracker is started.")
	// TODO clean old tempfiles
	err = t.server.Serve(listener)
	if err == http.ErrServerClosed {
		t.log.Notice("Tracker is shutting down.")
		return nil
	}
	return err
}

// Shutdown the tracker.
func (t *Tracker) Shutdown() error {
	timeout := time.Duration(t.config.Tracker.ShutdownTimeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_ = cancel
	err := t.server.Shutdown(ctx)
	if err != nil {
		t.log.Error("Error while shutting down HTTP server")
		return err
	}
	err = t.db.Close()
	if err != nil {
		t.log.Error("Error while closing database connection")
		return err
	}
	return nil
}

func (t *Tracker) ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong")) // nolint: errcheck
}

func (t *Tracker) getPaths(w http.ResponseWriter, r *http.Request) {
	var response struct {
		Paths []string `json:"paths"`
	}
	response.Paths = make([]string, 0)
	key := r.FormValue("key")
	rows, err := t.db.Query("select h.hostip, h.http_port, d.devid, f.fid from file f join file_on fo on f.fid=fo.fid join device d on d.devid=fo.devid join host h on h.hostid=d.hostid where f.dkey=? and f.dmid=?", key, dmid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var hostip string
		var httpPort int64
		var devid int64
		var fid int64
		err = rows.Scan(&hostip, &httpPort, &devid, &fid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sfid := fmt.Sprintf("%010d", fid)
		path := fmt.Sprintf("http://%s:%d/dev%d/%s/%s/%s/%s.fid", hostip, httpPort, devid, sfid[0:1], sfid[1:4], sfid[4:7], sfid)
		response.Paths = append(response.Paths, path)
	}
	err = rows.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

func (t *Tracker) createOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	var size uint64
	sizeStr := r.FormValue("size")
	if sizeStr == "" {
		size = 0
	} else {
		var err error
		size, err = strconv.ParseUint(r.FormValue("size"), 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	res, err := t.db.Exec("insert into tempfile(createtime, classid, dmid) values(?, ?, ?)", time.Now().UTC().Unix(), classid, dmid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, err := t.db.Query("select h.hostip, h.http_port, d.devid, (d.mb_total-d.mb_used) mb_free from device d join host h on d.hostid=h.hostid where h.status='alive' and d.status='alive' and (d.mb_total-d.mb_used)>= ? and mb_asof > ? order by mb_free desc", size, time.Now().UTC().Unix()-10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type device struct {
		hostip   string
		httpPort int64
		devid    int64
		mbFree   int64
	}
	devices := make([]device, 0)
	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var d device
		err = rows.Scan(&d.hostip, &d.httpPort, &d.devid, &d.mbFree)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		devices = append(devices, d)
	}
	err = rows.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fid, err := res.LastInsertId()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sfid := fmt.Sprintf("%010d", fid)
	if len(devices) == 0 {
		http.Error(w, "no device available", http.StatusNotFound)
		return
	}
	if len(devices) > 1 {
		devices = devices[:len(devices)/2]
	}
	d := devices[rand.Intn(len(devices))]
	response := struct {
		Path  string `json:"path"`
		Fid   int64  `json:"fid"`
		Devid int64  `json:"devid"`
	}{
		Path:  fmt.Sprintf("http://%s:%d/dev%d/%s/%s/%s/%s.fid", d.hostip, d.httpPort, d.devid, sfid[0:1], sfid[1:4], sfid[4:7], sfid),
		Fid:   fid,
		Devid: d.devid,
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

func (t *Tracker) createClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	fid, err := strconv.ParseInt(r.FormValue("fid"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	devid, err := strconv.ParseInt(r.FormValue("devid"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	size, err := strconv.ParseUint(r.FormValue("size"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	key := r.FormValue("key")
	if key == "" {
		http.Error(w, "required parameter: key", http.StatusBadRequest)
		return
	}
	tx, err := t.db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec("delete from tempfile where fid=?", fid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	count, err := res.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count == 0 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	_, err = tx.Exec("insert into file(fid, dkey, length, dmid, classid, devcount) values(?, ?, ?, ?, ?, 1)", fid, key, size, dmid, classid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = tx.Exec("insert into file_on(fid, devid) values(?, ?)", fid, devid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tx.Commit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (t *Tracker) deleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	key := r.FormValue("key")
	if key == "" {
		http.Error(w, "required parameter: key", http.StatusBadRequest)
		return
	}
	tx, err := t.db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	row := tx.QueryRow("select fid from file where dkey=? for update", key)
	var fid int64
	err = row.Scan(&fid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = tx.Exec("delete from file where fid=?", fid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = tx.Exec("delete from file_on where fid=?", fid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tx.Commit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO send task to host for removing from disk
}
