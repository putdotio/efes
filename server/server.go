package server

import (
	"database/sql"
	"fmt"
	"os"
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
	config           *config.Config
	dir              string
	devid            uint64
	db               *sql.DB
	log              log.Logger
	shutdown         chan struct{}
	diskStatsStopped chan struct{}
}

// New returns a new Server instance.
func New(c *config.Config, dir string) (*Server, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("Path must be a directory: %s", dir)
	}
	s := &Server{
		config:           c,
		dir:              dir,
		log:              log.NewLogger("server"),
		shutdown:         make(chan struct{}),
		diskStatsStopped: make(chan struct{}),
	}
	if s.config.Server.Debug {
		s.log.SetLevel(log.DEBUG)
	}
	s.devid, err = strconv.ParseUint(strings.TrimPrefix(filepath.Base(dir), "dev"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Cannot determine device ID from dir: %s", dir)
	}
	s.db, err = sql.Open("mysql", s.config.Database.DSN)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Run this server in a blocking manner. Running server can be stopped with Shutdown().
func (s *Server) Run() error {
	s.log.Notice("Server is started.")
	s.updateDiskStats()
	return nil
}

// Shutdown the server.
func (s *Server) Shutdown() error {
	close(s.shutdown)
	<-s.diskStatsStopped
	err := s.db.Close()
	if err != nil {
		s.log.Error("Error while closing database connection")
		return err
	}
	return nil
}

func (s *Server) updateDiskStats() {
	ticker := time.NewTicker(time.Second)
	iostat, err := newIOStat(s.dir)
	if err != nil {
		s.log.Warningln("Cannot get stats for dir:", s.dir)
	}
	for {
		select {
		case <-ticker.C:
			used, total := s.getDiskUsage()
			utilization := s.getDiskUtilization(iostat)
			_, err = s.db.Exec("update device set io_utilization=?, mb_used=?, mb_total=?, mb_asof=? where devid=?", utilization, used, total, time.Now().UTC().Unix(), s.devid)
			if err != nil {
				s.log.Errorln("Cannot update device stats:", err.Error())
				continue
			}
		case <-s.shutdown:
			close(s.diskStatsStopped)
			return
		}
	}
}

func (s *Server) getDiskUsage() (used, total sql.NullInt64) {
	usage, err := disk.Usage(s.dir)
	if err != nil {
		s.log.Errorln("Cannot get disk usage:", err.Error())
		return
	}
	const mb = 1 << 20
	used.Valid = true
	used.Int64 = int64(usage.Used) / mb
	total.Valid = true
	total.Int64 = int64(usage.Total) / mb
	s.log.Debugf("Disk usage: %d/%d", used.Int64, total.Int64)
	return
}

func (s *Server) getDiskUtilization(iostat *IOStat) (utilization sql.NullInt64) {
	if iostat == nil {
		return
	}
	value, err := iostat.Utilization()
	if err == errFirstRun {
		// We don't know the utilization level on the first call.
		return
	} else if err != nil {
		s.log.Errorln("Cannot get disk IO utilization:", err.Error())
		return
	}
	utilization.Valid = true
	utilization.Int64 = int64(value)
	s.log.Debugf("IO utilization: %d", utilization)
	return
}
