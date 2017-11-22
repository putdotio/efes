package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/log"
	"github.com/cenkalti/redialer/amqpredialer"
	"github.com/shirou/gopsutil/disk"
	"github.com/streadway/amqp"
)

// Server runs on storage servers.
type Server struct {
	deleteQueueName      string
	hostname             string
	config               *Config
	dir                  string
	devid                uint64
	db                   *sql.DB
	log                  log.Logger
	readServer           http.Server
	writeServer          http.Server
	shutdown             chan struct{}
	Ready                chan struct{}
	amqp                 *amqpredialer.AMQPRedialer
	onceDiskStatsUpdated sync.Once
	diskStatsUpdated     chan struct{}
	diskStatsStopped     chan struct{}
	diskCleanStopped     chan struct{}
	amqpRedialerStopped  chan struct{}
}

// NewServer returns a new Server instance.
func NewServer(c *Config) (*Server, error) {
	fi, err := os.Stat(c.Server.DataDir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("Path must be a directory: %s", c.Server.DataDir)
	}
	logger := log.NewLogger("server")
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	s := &Server{
		hostname:            hostname,
		deleteQueueName:     "delete_queue",
		config:              c,
		dir:                 c.Server.DataDir,
		log:                 logger,
		shutdown:            make(chan struct{}),
		Ready:               make(chan struct{}),
		diskStatsUpdated:    make(chan struct{}),
		diskStatsStopped:    make(chan struct{}),
		diskCleanStopped:    make(chan struct{}),
		amqpRedialerStopped: make(chan struct{}),
	}
	devicePrefix := "/" + filepath.Base(s.dir)
	s.writeServer.Handler = http.StripPrefix(devicePrefix, newFileReceiver(s.dir, s.log))
	s.readServer.Handler = http.StripPrefix(devicePrefix, http.FileServer(http.Dir(s.dir)))
	if s.config.Debug {
		s.log.SetLevel(log.DEBUG)
	}
	s.devid, err = strconv.ParseUint(strings.TrimPrefix(filepath.Base(s.dir), "dev"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Cannot determine device ID from dir: %s", s.dir)
	}
	s.db, err = sql.Open("mysql", s.config.Database.DSN)
	if err != nil {
		return nil, err
	}
	s.amqp, err = amqpredialer.New(c.AMQP.URL)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Run this server in a blocking manner. Running server can be stopped with Shutdown().
func (s *Server) Run() error {
	writeListener, err := net.Listen("tcp", s.config.Server.ListenAddress)
	if err != nil {
		return err
	}
	readListener, err := net.Listen("tcp", s.config.Server.ListenAddressForRead)
	if err != nil {
		return err
	}
	go s.cleanDisk()
	go s.updateDiskStats()
	go s.consumeDeleteQueue()
	s.log.Notice("Server is started.")
	errCh := make(chan error, 2)
	go func() {
		err = s.writeServer.Serve(writeListener)
		if err == http.ErrServerClosed {
			s.log.Notice("Write server is shutting down.")
		}
		errCh <- err
	}()
	go func() {
		err = s.readServer.Serve(readListener)
		if err == http.ErrServerClosed {
			s.log.Notice("Read server is shutting down.")
		}
		errCh <- err
	}()
	go func() {
		s.amqp.Run()
		close(s.amqpRedialerStopped)
	}()
	go s.notifyReady()
	err = <-errCh
	if err != nil {
		return err
	}
	err = <-errCh
	return err
}

func (s *Server) notifyReady() {
	select {
	case <-s.shutdown:
		return
	case <-s.diskStatsUpdated:
		close(s.Ready)
	}
}

// Shutdown the server.
func (s *Server) Shutdown() error {
	close(s.shutdown)

	timeout := time.Duration(s.config.Server.ShutdownTimeout) * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_ = cancel

	err := s.readServer.Shutdown(ctx)
	if err != nil {
		s.log.Error("Error while shutting down read HTTP server")
		return err
	}

	err = s.writeServer.Shutdown(ctx)
	if err != nil {
		s.log.Error("Error while shutting down write HTTP server")
		return err
	}

	<-s.diskStatsStopped
	<-s.diskCleanStopped

	err = s.db.Close()
	if err != nil {
		s.log.Error("Error while closing database connection")
		return err
	}
	err = s.amqp.Close()
	if err != nil {
		return err
	}
	<-s.amqpRedialerStopped
	return nil
}

func (s *Server) updateDiskStats() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	iostat, err := newIOStat(s.dir)
	if err != nil {
		s.log.Warningln("Cannot get stats for dir:", s.dir, "err:", err.Error())
	}
	for {
		select {
		case <-ticker.C:
			used, total := s.getDiskUsage()
			utilization := s.getDiskUtilization(iostat)
			_, err = s.db.Exec("update device set io_utilization=?, mb_used=?, mb_total=?, updated_at=current_timestamp where devid=?", utilization, used, total, s.devid)
			s.onceDiskStatsUpdated.Do(func() { close(s.diskStatsUpdated) })
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
	return
}

func (s *Server) cleanDisk() {
	var devid int64
	ticker := time.NewTicker(time.Minute * 1)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			row := s.db.QueryRow("select devid from device where devid=? and ADDDATE(last_disk_clean_time, INTERVAL ? SECOND) < CURRENT_TIMESTAMP", s.devid, s.config.Server.CleanDiskRunPeriod)
			err := row.Scan(&devid)
			if err != nil {
				if err == sql.ErrNoRows {
					continue
				} else {
					s.log.Error("Error getting last disk clean time", err)
					continue
				}
			}

			s.log.Debug("Updating disk clean time on db..")
			_, err = s.db.Exec("update device set last_disk_clean_time=current_timestamp where devid=?", s.devid)
			if err != nil {
				s.log.Error("Error during updating last disk clean time", err)
				continue
			}
			s.removeUnusedFids(s.dir)
		case <-s.shutdown:
			close(s.diskCleanStopped)
			return
		}
	}
}

func (s *Server) fidExistsOnDatabase(fileID int64) (bool, error) {
	var fid int
	existFile := true
	existTempFile := true
	// check file
	row := s.db.QueryRow("select fid from file where fid=?", fileID)
	err := row.Scan(&fid)
	if err != nil {
		if err == sql.ErrNoRows {
			existFile = false
		} else {
			s.log.Error("Error after querying file table ", err)
			return true, err
		}
	}
	// check tempfile
	row = s.db.QueryRow("select fid from tempfile where fid=?", fileID)
	err = row.Scan(&fid)
	if err != nil {
		if err == sql.ErrNoRows {
			existTempFile = false
		} else {
			s.log.Error("Error after querying tempfile table ", err)
			return true, err
		}
	}
	return existFile || existTempFile, nil

}

func (s *Server) removeUnusedFids(root string) {
	s.log.Debug("Remove unused fids started..")
	err := filepath.Walk(root, s.visitFiles)
	if err != nil {
		s.log.Error("Error during walk through files", err)
	}

}

func (s *Server) shouldDeleteFile(fileID int64, fileModtime time.Time) bool {
	existsOnDB, err := s.fidExistsOnDatabase(fileID)
	if err != nil {
		s.log.Error("Can not querying database ", err)
		return false
	}
	if existsOnDB {
		return false
	}
	ttl := time.Duration(s.config.Server.CleanDiskFileTTL) * time.Second
	return time.Now().Sub(fileModtime) > ttl
}

func (s *Server) visitFiles(path string, f os.FileInfo, err error) error {
	select {
	case <-s.shutdown:
		return io.EOF
	default:
		if filepath.Ext(path) != ".fid" {
			return nil
		}
		// Example file name: 0000000789.fid
		fileName := strings.Split(f.Name(), ".")
		fileID, err := strconv.ParseInt(strings.TrimLeft(fileName[0], "0,"), 10, 64)
		if err != nil {
			s.log.Error("Can not parse file name ", err)
			return nil
		}
		if s.shouldDeleteFile(fileID, f.ModTime()) {
			// TODO: Add delete logic.
			s.log.Infof("Fid %d is too old and there is no record on DB for it. Deleting...", fileID)
			err = s.publishDeleteTask(fileID)
			if err != nil {
				s.log.Error("Error while publishing delete task", err)
			}
			return nil
		}
	}
	return nil
}

func (s *Server) publishDeleteTask(fileID int64) error {
	conn, ok := <-s.amqp.Conn()
	if !ok {
		s.log.Infof("Failed to create amqp connection for publishing delete task")
		return amqp.ErrClosed
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	q, err := s.declareDeleteQueue(ch)
	if err != nil {
		return err
	}

	body := strconv.FormatInt(fileID, 10)
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	return err

}
func (s *Server) declareDeleteQueue(ch *amqp.Channel) (amqp.Queue, error) {
	q, err := ch.QueueDeclare(
		s.deleteQueueName, // name
		true,              // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		return amqp.Queue{}, err
	}
	return q, nil
}

func (s *Server) consumeDeleteQueue() {
	s.log.Debug("Starting delete queue consumer..")
	for {
		select {
		case <-s.shutdown:
			s.log.Notice("AMQP connection is shutting down..")
			return
		default:
		}

		conn, ok := <-s.amqp.Conn()
		if !ok {
			s.log.Error("Failed to create amqp connection for consuming delete queue.")
			continue
		}
		ch, err := conn.Channel()
		if err != nil {
			s.log.Error("Failed to open a channel for consuming delete task.", err)
			continue
		}

		q, err := s.declareDeleteQueue(ch)
		if err != nil {
			s.log.Infof("Failed to declare a queue", err)
			continue
		}

		pid := os.Getpid()
		consumerTag := "efes-delete-worker:" + strconv.Itoa(pid) + "@" + s.hostname + "/" + strconv.FormatUint(s.devid, 10)
		messages, err := ch.Consume(
			q.Name,      // queue
			consumerTag, // consumer
			false,       // auto-ack
			false,       // exclusive
			false,       // no-local
			false,       // no-wait
			nil,         // args
		)
		for msg := range messages {
			msgBody := string(msg.Body)
			fileID, err := strconv.ParseInt(msgBody, 10, 64)
			if err != nil {
				s.log.Error("Error while parsing int", err)
			}

			err = s.deleteFidOnDisk(fileID)
			if err != nil {
				s.log.Errorf("Failed to delete fid %d, %s", fileID, err)
				if err := msg.Nack(false, true); err != nil {
					s.log.Errorf("NACK error: %s", err)
				}
			}
			err = msg.Ack(false)
			if err != nil {
				s.log.Errorf("ACK error: %s", err)
			}

		}
	}
}

func (s *Server) deleteFidOnDisk(fileID int64) error {
	s.log.Debug("Deleting fid on disk ", fileID)
	sfid := fmt.Sprintf("%010d", fileID)
	path := fmt.Sprintf("%s/%s/%s/%s/%s.fid", s.dir, sfid[0:1], sfid[1:4], sfid[4:7], sfid)

	err := os.Remove(path)

	if err != nil {
		return err
	}
	s.log.Debugf("Fid %d deleted. ", fileID)
	return nil

}

func (s *Server) getDeleteQueueName() string {
	return s.deleteQueueName + "." + s.hostname

}
