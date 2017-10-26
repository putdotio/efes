package server

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	// Register MySQL database driver.
	_ "github.com/go-sql-driver/mysql"

	"github.com/cenkalti/log"
	"github.com/putdotio/efes/config"
	"github.com/shirou/gopsutil/disk"
)

// Server runs on storage servers.
type Server struct {
	config   *config.Config
	dir      string
	devid    uint64
	db       *sql.DB
	log      log.Logger
	stop     chan struct{}
	shutdown chan struct{}
}

// New returns a new Server instance.
func New(c *config.Config, dir string) (*Server, error) {
	devid, err := strconv.ParseUint(strings.TrimPrefix(filepath.Base(dir), "dev"), 10, 32)
	if err != nil {
		return nil, err
	}
	s := &Server{
		config:   c,
		dir:      dir,
		devid:    devid,
		log:      log.NewLogger("server"),
		stop:     make(chan struct{}),
		shutdown: make(chan struct{}),
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
	s.log.Notice("Server is started.")
	s.work()
	return nil
}

// Shutdown the server.
func (s *Server) Shutdown() error {
	close(s.stop)
	<-s.shutdown
	err := s.db.Close()
	if err != nil {
		s.log.Error("Error while closing database connection")
		return err
	}
	return nil
}

func (s *Server) work() {
	ticker := time.NewTicker(time.Second)
	var du diskUtilization
	for {
		select {
		case t := <-ticker.C:
			var dbUtil sql.NullInt64
			utilization, err := du.get(s.dir, t)
			if err != nil {
				s.log.Errorln("Cannot get disk IO utilization:", err.Error())
			} else {
				dbUtil.Valid = true
				dbUtil.Int64 = int64(utilization)
			}
			s.log.Debugf("dev%d IO utilization: %d", s.devid, utilization)

			var dbUsed, dbTotal sql.NullInt64
			usage, err := disk.Usage(s.dir)
			if err == errFirstRun {
				// We don't know the utilization level on the first call.
			} else if err != nil {
				s.log.Errorln("Cannot get disk usage:", err.Error())
			} else {
				const mb = 1 << 20
				dbUsed.Valid = true
				dbUsed.Int64 = int64(usage.Used) / mb
				dbTotal.Valid = true
				dbTotal.Int64 = int64(usage.Total) / mb
			}

			_, err = s.db.Exec("update device set io_utilization=?, mb_used=?, mb_total=?, mb_asof=? where devid=?", dbUtil, dbUsed, dbTotal, time.Now().UTC().Unix(), s.devid)
			if err != nil {
				s.log.Errorln("Cannot update device stats:", err.Error())
				continue
			}
		case <-s.stop:
			close(s.shutdown)
			return
		}
	}
}

type diskUtilization struct {
	io0 uint64
	t0  time.Time
}

var errFirstRun = errors.New("first utilization call")

func (d *diskUtilization) get(dir string, t time.Time) (percent uint8, err error) {
	dev, err := findDevice(dir)
	if err != nil {
		return
	}

	r, err := disk.IOCounters()
	if err != nil {
		return
	}

	c, ok := r[filepath.Base(dev)]
	if !ok {
		return
	}
	if d.t0.IsZero() {
		d.io0 = c.IoTime
		d.t0 = t
		err = errFirstRun
		return
	}
	diffIO := time.Duration(c.IoTime-d.io0) * time.Millisecond
	d.io0 = c.IoTime

	diffTime := t.Sub(d.t0)
	d.t0 = t

	percent = uint8((100 * diffIO) / diffTime)
	return
}

func findDevice(path string) (string, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return "", err
	}
	// fmt.Println("partitions:", partitions)
	for _, p := range partitions {
		_, err = filepath.Rel(p.Mountpoint, path)
		if err == nil {
			return p.Device, nil
		}
	}
	return "", fmt.Errorf("Device could not be found: %s", path)
}
