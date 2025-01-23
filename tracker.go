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
	"strings"
	"time"

	"github.com/cenkalti/log"
	"github.com/cenkalti/redialer/amqpredialer"
	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Tracker tracks the info of files in database.
// Tracker responds to client requests.
// Tracker sends jobs to servers.
type Tracker struct {
	config                 *Config
	db                     *sql.DB
	log                    log.Logger
	server                 http.Server
	metricsServer          http.Server
	amqp                   *amqpredialer.AMQPRedialer
	shutdown               chan struct{}
	Ready                  chan struct{}
	tempfileCleanerStopped chan struct{}
	amqpRedialerStopped    chan struct{}
}

// NewTracker returns a new Tracker instance.
func NewTracker(c *Config) (*Tracker, error) {
	t := &Tracker{
		config:                 c,
		log:                    log.NewLogger("tracker"),
		shutdown:               make(chan struct{}),
		Ready:                  make(chan struct{}),
		tempfileCleanerStopped: make(chan struct{}),
		amqpRedialerStopped:    make(chan struct{}),
	}
	m := http.NewServeMux()
	m.HandleFunc("/ping", t.ping)
	m.HandleFunc("/get-path", t.getPath)
	m.HandleFunc("/get-paths", t.getPaths)
	m.HandleFunc("/get-devices", t.getDevices)
	m.HandleFunc("/get-hosts", t.getHosts)
	m.HandleFunc("/get-racks", t.getRacks)
	m.HandleFunc("/get-zones", t.getZones)
	m.HandleFunc("/create-open", t.createOpen)
	m.HandleFunc("/create-close", t.createClose)
	m.HandleFunc("/delete", t.deleteFile)
	m.HandleFunc("/iter-files", t.iterFiles)

	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic:         false,
		WaitForDelivery: true,
	})

	// main server
	t.server = http.Server{
		Handler:      sentryHandler.HandleFunc(addVersion(m)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	t.metricsServer = http.Server{
		Handler: metricsMux,
	}

	if t.config.Debug {
		t.log.SetLevel(log.DEBUG)
	}
	var err error
	t.db, err = openDatabase(c.Database)
	if err != nil {
		return nil, err
	}
	t.amqp, err = amqpredialer.New(c.AMQP.URL)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func addVersion(h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("efes-version", Version)
		h.ServeHTTP(w, r)
	})
}

// Run this tracker in a blocking manner. Running tracker can be stopped with Shutdown().
func (t *Tracker) Run() error {
	listener, err := net.Listen("tcp", t.config.Tracker.ListenAddress)
	if err != nil {
		return err
	}
	metricsListener, err := net.Listen("tcp", t.config.Tracker.ListenAddressForMetrics)
	if err != nil {
		return err
	}
	t.log.Notice("Starting tempfile cleaner...")
	go t.tempfileCleaner()
	go func() {
		t.log.Notice("Running amqp redialer...")
		t.amqp.Run()
		close(t.amqpRedialerStopped)
	}()
	close(t.Ready)

	errCh := make(chan error, 2)
	go func() {
		t.log.Noticef("Starting metrics server on %v", metricsListener.Addr())
		err2 := t.metricsServer.Serve(metricsListener)
		if err2 == http.ErrServerClosed {
			t.log.Notice("Metrics server is shutting down.")
		}
		errCh <- err2
	}()
	go func() {
		t.log.Noticef("Starting HTTP server on %v", listener.Addr())
		err2 := t.server.Serve(listener)
		if err2 == http.ErrServerClosed {
			t.log.Notice("Read server is shutting down.")
		}
		errCh <- err2
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}
	return nil
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

	err = t.metricsServer.Shutdown(ctx)
	if err != nil {
		t.log.Error("Error while shutting down metrics HTTP server")
		return err
	}

	<-t.tempfileCleanerStopped
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
	sentry.CaptureException(err)
	sentry.CaptureMessage(message)
	message = message + ": " + err.Error()
	t.log.Error(message + "; " + r.URL.Path)
	http.Error(w, message, http.StatusInternalServerError)
}

func (t *Tracker) getPath(w http.ResponseWriter, r *http.Request) {
	var response GetPath
	key := r.FormValue("key")
	row := t.db.QueryRowContext(r.Context(), "select h.hostname, d.read_port, d.devid, f.fid, f.created_at "+
		"from file f "+
		"join file_on fo on f.fid=fo.fid "+
		"join device d on d.devid=fo.devid "+
		"join host h on h.hostid=d.hostid "+
		"where h.status='alive' "+
		"and d.status in ('alive', 'drain') "+
		"and f.dkey=?", key)
	var hostname string
	var httpPort int64
	var devid int64
	var fid int64
	var createdAt sql.NullTime
	err := row.Scan(&hostname, &httpPort, &devid, &fid, &createdAt)
	if err == sql.ErrNoRows {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if err != nil {
		t.internalServerError("cannot scan rows", err, r, w)
		return
	}
	w.Header().Set("content-type", "application/json")
	response.Path = fmt.Sprintf("http://%s:%d/dev%d/%s", hostname, httpPort, devid, vivify(fid))
	response.CreatedAt = createdAt.Time.Format(time.RFC3339)
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

func (t *Tracker) getPaths(w http.ResponseWriter, r *http.Request) {
	response := GetPaths{
		Paths: make([]GetPath, 0),
	}
	key := r.FormValue("key")
	rows, err := t.db.QueryContext(r.Context(), "select h.hostname, d.read_port, d.devid, f.fid, f.created_at "+
		"from file f "+
		"join file_on fo on f.fid=fo.fid "+
		"join device d on d.devid=fo.devid "+
		"join host h on h.hostid=d.hostid "+
		"where h.status='alive' "+
		"and d.status in ('alive', 'drain') "+
		"and f.dkey=?", key)
	if err != nil {
		t.internalServerError("cannot select paths", err, r, w)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var hostname string
		var httpPort int64
		var devid int64
		var fid int64
		var createdAt sql.NullTime
		err = rows.Scan(&hostname, &httpPort, &devid, &fid, &createdAt)
		if err == sql.ErrNoRows {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		if err != nil {
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		path := GetPath{
			Path:      fmt.Sprintf("http://%s:%d/dev%d/%s", hostname, httpPort, devid, vivify(fid)),
			CreatedAt: createdAt.Time.Format(time.RFC3339),
		}
		response.Paths = append(response.Paths, path)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("cannot scan rows", err, r, w)
		return
	}
	w.Header().Set("content-type", "application/json")
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
	d, err := findAliveDevice(t.db, int64(size), nil, getClientIP(r))
	if err == errNoDeviceAvailable {
		http.Error(w, "no device available", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		t.internalServerError("cannot find a device", err, r, w)
		return
	}
	res, err := t.db.ExecContext(r.Context(), "insert into tempfile(devid) values(?)", d.devid)
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
		Path: d.PatchURL(fid),
		Fid:  fid,
	}
	w.Header().Set("content-type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

var errNoDeviceAvailable = errors.New("no device available")

type aliveDevice struct {
	zoneid   int64
	rackid   int64
	hostid   int64
	hostip   string
	hostname string
	httpPort int64
	devid    int64
}

func (d *aliveDevice) PatchURL(fid int64) string {
	return fmt.Sprintf("http://%s:%d/dev%d/%s", d.hostname, d.httpPort, d.devid, vivify(fid))
}

func findAliveDevice(db *sql.DB, size int64, devids []int64, clientIP string) (*aliveDevice, error) {
	var devidsSQL string
	if len(devids) > 0 {
		var devidsString []string
		for _, devid := range devids {
			devidsString = append(devidsString, strconv.FormatInt(devid, 10))
		}
		devidsSQL = "and d.status in ('alive', 'drain') and d.devid in (" + strings.Join(devidsString, ",") + ") "
	} else {
		devidsSQL = "and d.status='alive' "
	}
	rows, err := db.Query("select z.zoneid, r.rackid, h.hostid, h.hostip, h.hostname, d.devid, d.write_port "+ // nolint: gosec
		"from device d "+
		"join host h on d.hostid=h.hostid "+
		"join rack r on h.rackid=r.rackid "+
		"join zone z on r.zoneid=z.zoneid "+
		"where h.status='alive' "+
		"and bytes_free>= ? "+
		devidsSQL+
		"and timestampdiff(second, updated_at, current_timestamp) < 60 "+
		"order by bytes_free desc", size)
	if err != nil {
		return nil, err
	}
	devices := make([]aliveDevice, 0)
	defer rows.Close()
	for rows.Next() {
		var d aliveDevice
		err = rows.Scan(&d.zoneid, &d.rackid, &d.hostid, &d.hostip, &d.hostname, &d.devid, &d.httpPort)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	sameHostDevices := filterSameHost(devices, clientIP)
	if len(sameHostDevices) > 0 { // nolint: nestif
		devices = sameHostDevices
	} else {
		subnets, err := getSubnets(db)
		if err != nil {
			return nil, err
		}
		rackID, zoneID, ok := getRackID(subnets, clientIP)
		if ok {
			sameRackDevices := filterSameRack(devices, rackID)
			if len(sameRackDevices) > 0 {
				devices = sameRackDevices
			} else {
				sameZoneDevices := filterSameZone(devices, zoneID)
				if len(sameZoneDevices) > 0 {
					devices = sameZoneDevices
				}
			}
		}
	}
	if len(devices) == 0 {
		return nil, errNoDeviceAvailable
	}
	if len(devices) == 1 {
		return &devices[0], nil
	}
	devices = devices[:len(devices)/2]
	return &devices[rand.Intn(len(devices))], nil // nolint: gosec
}

func getRackID(subnets []subnet, clientIP string) (rackid, zoneid int64, ok bool) {
	ip := net.ParseIP(clientIP)
	for _, s := range subnets {
		_, ipnet, err := net.ParseCIDR(s.subnet)
		if err != nil {
			continue
		}
		if ipnet.Contains(ip) {
			return s.rackid, s.zoneid, true
		}
	}
	return 0, 0, false
}

func filterSameRack(devices []aliveDevice, rackID int64) []aliveDevice {
	var ret []aliveDevice
	for _, d := range devices {
		if d.rackid == rackID {
			ret = append(ret, d)
		}
	}
	return ret
}

func filterSameZone(devices []aliveDevice, zoneID int64) []aliveDevice {
	var ret []aliveDevice
	for _, d := range devices {
		if d.zoneid == zoneID {
			ret = append(ret, d)
		}
	}
	return ret
}

func getClientIP(req *http.Request) string {
	xff := req.Header.Get("x-forwarded-for")
	if xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return req.RemoteAddr
}

func filterSameHost(devices []aliveDevice, clientIP string) []aliveDevice {
	var ret []aliveDevice
	for _, d := range devices {
		if d.hostip == clientIP {
			ret = append(ret, d)
		}
	}
	return ret
}

type subnet struct {
	subnetid int64
	rackid   int64
	zoneid   int64
	subnet   string
}

func getSubnets(db *sql.DB) ([]subnet, error) {
	var ret []subnet
	rows, err := db.Query("select subnetid, r.rackid, z.zoneid, subnet from subnet s join rack r on s.rackid=r.rackid join zone z on z.zoneid=r.zoneid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s subnet
		err = rows.Scan(&s.subnetid, &s.rackid, &s.zoneid, &s.subnet)
		if err != nil {
			return nil, err
		}
		ret = append(ret, s)
	}
	return ret, rows.Err()
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
	key := r.FormValue("key")
	if key == "" {
		http.Error(w, "required parameter: key", http.StatusBadRequest)
		return
	}
	tx, err := t.db.BeginTx(r.Context(), nil)
	if err != nil {
		t.internalServerError("cannot begin transaction", err, r, w)
		return
	}
	defer tx.Rollback() // nolint: errcheck
	var devid int64
	row := tx.QueryRow("select devid from tempfile where fid=? for update", fid)
	err = row.Scan(&devid)
	if err == sql.ErrNoRows {
		http.Error(w, "no tempfile found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "duplicate create-close call", http.StatusConflict)
		return
	}
	_, err = tx.Exec("delete from tempfile where fid=?", fid)
	if err != nil {
		t.internalServerError("cannot delete tempfile", err, r, w)
		return
	}
	// Remove existing fids with same dkey if there is any.
	var oldfid int64
	var olddevids []int64
	row = tx.QueryRow("select fid from file where dkey=? for update", key)
	err = row.Scan(&oldfid)
	switch err {
	case sql.ErrNoRows:
	case nil:
		olddevids, err = t.deleteFidOnDB(tx, oldfid)
		if err != nil {
			t.internalServerError("cannot delete fid", err, r, w)
			return
		}
	default:
		t.internalServerError("cannot select old fid record", err, r, w)
		return
	}
	// After removing old fids above, a new fid may come with the same dkey.
	// Use REPLACE INTO feature of MySQL to prevent "duplicate entry" errors.
	// This is not thread-safe and may result stale "file_on" records with no fid present in "file" table.
	// It is a very rare case and cleanDevice() job will eventually remove stale records on "file_on" table.
	_, err = tx.Exec("replace into file(fid, dkey, created_at) values(?,?,now())", fid, key)
	if err != nil {
		t.internalServerError("cannot insert or replace file", err, r, w)
		return
	}
	_, err = tx.Exec("insert into file_on(fid, devid) values(?, ?)", fid, devid)
	if err != nil {
		t.internalServerError("cannot insert file_on record", err, r, w)
		return
	}
	row = tx.QueryRow("select h.hostname, d.read_port "+
		"from device d join host h on h.hostid=d.hostid "+
		"where d.devid=?", devid)
	var hostname string
	var httpPort int64
	err = row.Scan(&hostname, &httpPort)
	if err != nil {
		t.internalServerError("cannot select host ip", err, r, w)
		return
	}
	err = tx.Commit()
	if err != nil {
		t.internalServerError("cannot commit transaction", err, r, w)
		return
	}
	if olddevids != nil {
		go t.publishDeleteTask(olddevids, oldfid)
	}
	w.Header().Set("content-type", "application/json")
	var response CreateClose
	response.Path = fmt.Sprintf("http://%s:%d/dev%d/%s", hostname, httpPort, devid, vivify(fid))
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

func (t *Tracker) deleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	key := r.FormValue("key")
	fid, _ := strconv.ParseInt(r.FormValue("fid"), 10, 64)
	if key == "" && fid == 0 {
		http.Error(w, "required parameter: key or fid", http.StatusBadRequest)
		return
	}
	tx, err := t.db.BeginTx(r.Context(), nil)
	if err != nil {
		t.internalServerError("cannot begin transaction", err, r, w)
		return
	}
	defer tx.Rollback() // nolint: errcheck
	if fid == 0 {
		row := tx.QueryRow("select fid from file where dkey=? for update", key)
		err = row.Scan(&fid)
		if err == sql.ErrNoRows {
			return
		}
		if err != nil {
			t.internalServerError("cannot select rows", err, r, w)
			return
		}
	}
	devids, err := t.deleteFidOnDB(tx, fid)
	if err != nil {
		t.internalServerError("cannot delete fid", err, r, w)
		return
	}
	err = tx.Commit()
	if err != nil {
		t.internalServerError("cannot commit transaction", err, r, w)
		return
	}
	go t.publishDeleteTask(devids, fid)
}

func (t *Tracker) deleteFidOnDB(tx *sql.Tx, fid int64) (devids []int64, err error) {
	devids, err = getDevicesOfFid(tx, fid)
	if err != nil {
		return
	}
	_, err = tx.Exec("delete from file_on where fid=?", fid)
	if err != nil {
		return
	}
	_, err = tx.Exec("delete from file where fid=?", fid)
	if err != nil {
		return
	}
	return
}

func (t *Tracker) publishDeleteTask(devids []int64, fid int64) {
	select {
	case conn, ok := <-t.amqp.Conn():
		if !ok {
			t.log.Error("Cannot publish delete task. AMQP connection is closed.")
			return
		}
		ch, err := conn.Channel()
		if err != nil {
			t.log.Errorln("cannot open amqp channel:", err.Error())
			return
		}
		for _, devid := range devids {
			err = publishDeleteTask(ch, devid, fid)
			if err != nil {
				t.log.Errorln("cannot publish delete task:", err.Error())
			}
		}
		err = ch.Close()
		if err != nil {
			t.log.Errorln("cannot close amqp channel:", err.Error())
		}
	case <-t.shutdown:
		t.log.Warningf("Not sending delete task for fid=%d because shutdown is requested while waiting for amqp connection", fid)
	}
}

func publishDeleteTask(ch *amqp.Channel, devid int64, fileID int64) error {
	body := strconv.FormatInt(fileID, 10)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	return ch.PublishWithContext(
		ctx,
		"",                     // exchange
		deleteQueueName(devid), // routing key
		false,                  // mandatory
		false,                  // immediate
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
	defer rows.Close()
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

func (t *Tracker) getDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var err error
	devices := make([]Device, 0)

	rows, err := t.db.QueryContext(r.Context(), "select d.devid, d.hostid, h.hostname, h.status, h.rackid, r.name, r.zoneid, z.name, d.status, d.bytes_total, d.bytes_used, d.bytes_free, unix_timestamp(d.updated_at), d.io_utilization from device d join host h on h.hostid=d.hostid join rack r on r.rackid=h.rackid join zone z on z.zoneid=r.zoneid")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close()
	for rows.Next() {
		var d Device
		var bytesTotal, bytesUsed, bytesFree sql.NullInt64
		var ioUtilization sql.NullInt64
		err = rows.Scan(&d.Devid, &d.Hostid, &d.HostName, &d.HostStatus, &d.Rackid, &d.RackName, &d.Zoneid, &d.ZoneName, &d.Status, &bytesTotal, &bytesUsed, &bytesFree, &d.UpdatedAt, &ioUtilization)
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

	w.Header().Set("content-type", "application/json")
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
	rows, err = t.db.QueryContext(r.Context(), "select hostid, status, hostname, hostip from host")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close()
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

	w.Header().Set("content-type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

func (t *Tracker) getRacks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var err error
	racks := make([]Rack, 0)

	var rows *sql.Rows
	rows, err = t.db.QueryContext(r.Context(), "select rackid, zoneid, name from rack")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close()
	for rows.Next() {
		var ra Rack
		err = rows.Scan(&ra.Rackid, &ra.Zoneid, &ra.Name)
		if err != nil {
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		racks = append(racks, ra)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("error while fetching rows", err, r, w)
		return
	}

	var response GetRacks
	response.Racks = racks

	w.Header().Set("content-type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}

func (t *Tracker) getZones(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var err error
	zones := make([]Zone, 0)

	var rows *sql.Rows
	rows, err = t.db.QueryContext(r.Context(), "select zoneid, name from zone")
	if err != nil {
		t.internalServerError("cannot select rows", err, r, w)
		return
	}

	defer rows.Close()
	for rows.Next() {
		var z Zone
		err = rows.Scan(&z.Zoneid, &z.Name)
		if err != nil {
			t.internalServerError("cannot scan rows", err, r, w)
			return
		}
		zones = append(zones, z)
	}
	err = rows.Err()
	if err != nil {
		t.internalServerError("error while fetching rows", err, r, w)
		return
	}

	var response GetZones
	response.Zones = zones

	w.Header().Set("content-type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(response) // nolint: errcheck
}
