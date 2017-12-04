package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/log"
	"github.com/cenkalti/redialer/amqpredialer"
	"github.com/streadway/amqp"
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
	Ready                     chan struct{}
	removeOldTempfilesStopped chan struct{}
	amqpRedialerStopped       chan struct{}
}

// NewTracker returns a new Tracker instance.
func NewTracker(c *Config) (*Tracker, error) {
	t := &Tracker{
		config:   c,
		log:      log.NewLogger("tracker"),
		shutdown: make(chan struct{}),
		Ready:    make(chan struct{}),
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
	close(t.Ready)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.config.Tracker.ShutdownTimeout))
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
	rows, err := t.db.Query("select h.hostip, d.read_port, d.devid, f.fid "+
		"from file f "+
		"join file_on fo on f.fid=fo.fid "+
		"join device d on d.devid=fo.devid "+
		"join host h on h.hostid=d.hostid "+
		"where h.status='alive' "+
		"and d.status in ('alive', 'drain') "+
		"and f.dkey=?", key)
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
		path := fmt.Sprintf("http://%s:%d/dev%d/%s", hostip, httpPort, devid, vivify(fid))
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
	d, err := findAliveDevice(t.db, int64(size))
	if err == errNoDeviceAvailable {
		http.Error(w, "no device available", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		t.internalServerError("cannot find a device", err, r, w)
		return
	}
	res, err := t.db.Exec("insert into tempfile(createtime) values(UNIX_TIMESTAMP())")
	if err != nil {
		t.internalServerError("cannot insert tempfile", err, r, w)
		return
	}
	fid, err := res.LastInsertId()
	if err != nil {
		t.internalServerError("cannot get last insert id", err, r, w)
		return
	}
	response := CreateOpen{
		Path:  d.PatchURL(fid),
		Fid:   fid,
		Devid: d.devid,
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

var errNoDeviceAvailable = errors.New("no device available")

type aliveDevice struct {
	hostip   string
	httpPort int64
	devid    int64
}

func (d *aliveDevice) PatchURL(fid int64) string {
	return fmt.Sprintf("http://%s:%d/dev%d/%s", d.hostip, d.httpPort, d.devid, vivify(fid))
}

func findAliveDevice(db *sql.DB, size int64) (*aliveDevice, error) {
	rows, err := db.Query("select h.hostip, d.write_port, d.devid "+
		"from device d "+
		"join host h on d.hostid=h.hostid "+
		"where h.status='alive' "+
		"and d.status='alive' "+
		"and bytes_free>= ? "+
		"and timestampdiff(second, updated_at, current_timestamp) < 60 "+
		"order by bytes_free desc", size)
	if err != nil {
		return nil, err
	}
	devices := make([]aliveDevice, 0)
	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var d aliveDevice
		err = rows.Scan(&d.hostip, &d.httpPort, &d.devid)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, errNoDeviceAvailable
	}
	if len(devices) == 1 {
		return &devices[0], nil
	}
	devices = devices[:len(devices)/2]
	return &devices[rand.Intn(len(devices))], nil
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

	_, err = tx.Exec("replace into file(fid, dkey) values(?,?)", fid, key)
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
	row := tx.QueryRow("select fid from file where dkey=? for update", key)
	var fid int64
	err = row.Scan(&fid)
	if err == sql.ErrNoRows {
		return
	}
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}
	devids, err := getDevicesOfFid(tx, fid)
	if err != nil {
		t.internalServerError("cannot get devices of fid", err, r, w)
		return
	}
	_, err = tx.Exec("delete from file_on where fid=?", fid)
	if err != nil {
		t.internalServerError("cannot delete file_on records", err, r, w)
		return
	}
	_, err = tx.Exec("delete from file where fid=?", fid)
	if err != nil {
		t.internalServerError("cannot delete file", err, r, w)
		return
	}
	err = tx.Commit()
	if err != nil {
		t.internalServerError("cannot commit transaction", err, r, w)
		return
	}
	go t.publishDeleteTask(devids, fid)
}

func (t *Tracker) publishDeleteTask(devids []int64, fid int64) {
	select {
	case conn, ok := <-t.amqp.Conn():
		if !ok {
			log.Error("Cannot publish delete task. AMQP connection is closed.")
			return
		}
		ch, err := conn.Channel()
		if err != nil {
			log.Errorln("cannot open amqp channel:", err.Error())
			return
		}
		defer logCloseAMQPChannel(t.log, ch)
		for _, devid := range devids {
			err = publishDeleteTask(ch, devid, fid)
			if err != nil {
				log.Errorln("cannot publish delete task:", err.Error())
			}
		}
	case <-t.shutdown:
		t.log.Warningf("Not sending delete task for fid=%d because shutdown is requested while waiting for amqp connection", fid)
	}
}

func publishDeleteTask(ch *amqp.Channel, devid int64, fileID int64) error {
	body := strconv.FormatInt(fileID, 10)
	return ch.Publish(
		"", // exchange
		deleteQueueName(devid), // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
}

func getDevicesOfFid(tx *sql.Tx, fid int64) (devids []int64, err error) {
	rows, err := tx.Query("select devid from file_on where fid=? for update", fid)
	if err != nil {
		return nil, err
	}
	devids = make([]int64, 0)
	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var devid int64
		err = rows.Scan(&devid)
		if err != nil {
			return nil, err
		}
		devids = append(devids, devid)
	}
	err = rows.Err()
	return
}

func (t *Tracker) removeOldTempfiles() {
	tempfileTooOld := time.Duration(t.config.Tracker.TempfileTooOld) / time.Second
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			res, err := t.db.Exec("delete from tempfile where createtime < UNIX_TIMESTAMP() - ?", tempfileTooOld)
			if err != nil {
				t.log.Errorln("cannot delete old tempfile records:", err.Error())
				continue
			}
			count, err := res.RowsAffected()
			if err == nil {
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
	devices := make([]Device, 0)

	rows, err := t.db.Query("select devid, hostid, status, bytes_total, bytes_used, bytes_free, unix_timestamp(updated_at), io_utilization from device")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var d Device
		var bytesTotal, bytesUsed, bytesFree sql.NullInt64
		var ioUtilization sql.NullInt64
		err = rows.Scan(&d.Devid, &d.Hostid, &d.Status, &bytesTotal, &bytesUsed, &bytesFree, &d.UpdatedAt, &ioUtilization)
		if err != nil {
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		if bytesTotal.Valid {
			d.BytesTotal = &bytesTotal.Int64
		}
		if bytesUsed.Valid {
			d.BytesUsed = &bytesUsed.Int64
		}
		if bytesFree.Valid {
			d.BytesFree = &bytesFree.Int64
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
	hosts := make([]Host, 0)

	var rows *sql.Rows
	rows, err = t.db.Query("select hostid, status, hostname, hostip from host")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var h Host
		err = rows.Scan(&h.Hostid, &h.Status, &h.Hostname, &h.HostIP)
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
