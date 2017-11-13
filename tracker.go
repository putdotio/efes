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
	"github.com/cenkalti/redialer/amqpredialer"
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
	config                    *Config
	db                        *sql.DB
	log                       log.Logger
	server                    http.Server
	amqp                      *amqpredialer.AMQPRedialer
	shutdown                  chan struct{}
	removeOldTempfilesStopped chan struct{}
	amqpRedialerStopped       chan struct{}
}

// NewTracker returns a new Tracker instance.
func NewTracker(c *Config) (*Tracker, error) {
	t := &Tracker{
		config:                    c,
		log:                       log.NewLogger("tracker"),
		shutdown:                  make(chan struct{}),
		removeOldTempfilesStopped: make(chan struct{}),
		amqpRedialerStopped:       make(chan struct{}),
	}
	m := http.NewServeMux()
	m.HandleFunc("/ping", t.ping)
	m.HandleFunc("/get-paths", t.getPaths)
	m.HandleFunc("/get-devices", t.getDevices)
	m.HandleFunc("/get-hosts", t.getHosts)
	m.HandleFunc("/create-open", t.createOpen)
	m.HandleFunc("/create-close", t.createClose)
	m.HandleFunc("/delete", t.deleteFile)
	t.server.Handler = m
	if t.config.Debug {
		t.log.SetLevel(log.DEBUG)
	}
	var err error
	t.db, err = sql.Open("mysql", t.config.Database.DSN)
	if err != nil {
		return nil, err
	}
	t.amqp, err = amqpredialer.New(c.AMQP.URL)
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
	go t.removeOldTempfiles()
	go func() {
		t.amqp.Run()
		close(t.amqpRedialerStopped)
	}()
	err = t.server.Serve(listener)
	if err == http.ErrServerClosed {
		t.log.Notice("Tracker is shutting down.")
		return nil
	}
	return err
}

// Shutdown the tracker.
func (t *Tracker) Shutdown() error {
	close(t.shutdown)

	timeout := time.Duration(t.config.Tracker.ShutdownTimeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_ = cancel
	err := t.server.Shutdown(ctx)
	if err != nil {
		t.log.Error("Error while shutting down HTTP server")
		return err
	}

	<-t.removeOldTempfilesStopped
	err = t.db.Close()
	if err != nil {
		t.log.Error("Error while closing database connection")
		return err
	}
	err = t.amqp.Close()
	if err != nil {
		return err
	}
	<-t.amqpRedialerStopped
	return nil
}

func (t *Tracker) ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong")) // nolint: errcheck
}

func (t *Tracker) internalServerError(message string, err error, r *http.Request, w http.ResponseWriter) {
	message = message + ": " + err.Error()
	t.log.Error(message + "; " + r.URL.Path)
	http.Error(w, message, http.StatusInternalServerError)
}

func (t *Tracker) getPaths(w http.ResponseWriter, r *http.Request) {
	var response GetPaths
	response.Paths = make([]string, 0)
	key := r.FormValue("key")
	rows, err := t.db.Query("select h.hostip, h.http_port, d.devid, f.fid from file f join file_on fo on f.fid=fo.fid join device d on d.devid=fo.devid join host h on h.hostid=d.hostid where f.dkey=? and f.dmid=?", key, dmid)
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
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
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		sfid := fmt.Sprintf("%010d", fid)
		path := fmt.Sprintf("http://%s:%d/dev%d/%s/%s/%s/%s.fid", hostip, httpPort, devid, sfid[0:1], sfid[1:4], sfid[4:7], sfid)
		response.Paths = append(response.Paths, path)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("error while fetching rows", err, r, w)
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
		size, err = strconv.ParseUint(sizeStr, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	res, err := t.db.Exec("insert into tempfile(createtime, classid, dmid) values(?, ?, ?)", time.Now().UTC().Unix(), classid, dmid)
	if err != nil {
		t.internalServerError("cannot insert tempfile", err, r, w)
		return
	}
	rows, err := t.db.Query("select h.hostip, d.http_port, d.devid, (d.mb_total-d.mb_used) mb_free from device d join host h on d.hostid=h.hostid where h.status='alive' and d.status='alive' and (d.mb_total-d.mb_used)>= ? and timestampdiff(second, updated_at, current_timestamp) < 60 order by mb_free desc", size)
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
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
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		devices = append(devices, d)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("error while fetching rows", err, r, w)
		return
	}
	fid, err := res.LastInsertId()
	if err != nil {
		t.internalServerError("cannot get last insert id", err, r, w)
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
	response := CreateOpen{
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
		t.internalServerError("cannot begin transaction", err, r, w)
		return
	}
	defer tx.Rollback() // nolint: errcheck
	res, err := tx.Exec("delete from tempfile where fid=?", fid)
	if err != nil {
		t.internalServerError("cannot delete tempfile", err, r, w)
		return
	}
	count, err := res.RowsAffected()
	if err != nil {
		t.internalServerError("cannot get rows affected", err, r, w)
		return
	}
	if count == 0 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	_, err = tx.Exec("replace into file(fid, dmid, dkey, length, classid, devcount) values(?,?,?,?,?,1)", fid, dmid, key, size, classid)
	if err != nil {
		t.internalServerError("cannot insert or replace file", err, r, w)
		return
	}
	_, err = tx.Exec("insert into file_on(fid, devid) values(?, ?)", fid, devid)
	if err != nil {
		t.internalServerError("cannot insert file_on record", err, r, w)
		return
	}
	err = tx.Commit()
	if err != nil {
		t.internalServerError("cannot commit transaction", err, r, w)
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
		t.internalServerError("cannot begin transaction", err, r, w)
		return
	}
	defer tx.Rollback() // nolint: errcheck
	row := tx.QueryRow("select fid from file where dkey=? and dmid=? for update", key, dmid)
	var fid int64
	err = row.Scan(&fid)
	if err == sql.ErrNoRows {
		return
	}
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}
	_, err = tx.Exec("delete from file where fid=?", fid)
	if err != nil {
		t.internalServerError("cannot delete file", err, r, w)
		return
	}
	_, err = tx.Exec("delete from file_on where fid=?", fid)
	if err != nil {
		t.internalServerError("cannot delete file_on records", err, r, w)
		return
	}
	err = tx.Commit()
	if err != nil {
		t.internalServerError("cannot commit transaction", err, r, w)
		return
	}
	// TODO send task to host for removing from disk
}

func (t *Tracker) removeOldTempfiles() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			tooOld := time.Duration(t.config.Tracker.TempfileTooOld) * time.Millisecond
			deadline := now.Add(-tooOld)
			res, err := t.db.Exec("delete from tempfile where createtime < ?", deadline)
			if err != nil {
				t.log.Errorln("cannot delete old tempfile records:", err.Error())
				continue
			}
			count, err := res.RowsAffected()
			if err != nil {
				t.log.Infoln(count, "old tempfile records are deleted")
			}
		case <-t.shutdown:
			close(t.removeOldTempfilesStopped)
			return
		}
	}
}

func (t *Tracker) getDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var err error

	type device Device
	devices := make([]Device, 0)

	rows, err := t.db.Query("select devid, hostid, status, mb_total, mb_used, unix_timestamp(updated_at), io_utilization from device")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var d Device
		var mbTotal sql.NullInt64
		var mbUsed sql.NullInt64
		var ioUtilization sql.NullInt64
		err = rows.Scan(&d.Devid, &d.Hostid, &d.Status, &mbTotal, &mbUsed, &d.MbAsof, &ioUtilization)
		if err != nil {
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		if mbTotal.Valid {
			d.MbTotal = &mbTotal.Int64
		}
		if mbUsed.Valid {
			d.MbUsed = &mbUsed.Int64
		}
		if ioUtilization.Valid {
			d.IoUtilization = &ioUtilization.Int64
		}
		devices = append(devices, d)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("error while fetching rows", err, r, w)
		return
	}

	var response GetDevices
	response.Devices = devices

	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

func (t *Tracker) getHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var err error

	type host Host
	hosts := make([]Host, 0)

	var rows *sql.Rows
	rows, err = t.db.Query("select hostid, status, http_port, hostname, hostip from host")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var h Host
		err = rows.Scan(&h.Hostid, &h.Status, &h.HTTPPort, &h.Hostname, &h.HostIP)
		if err != nil {
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		hosts = append(hosts, h)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("error while fetching rows", err, r, w)
		return
	}

	var response GetHosts
	response.Hosts = hosts

	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}
