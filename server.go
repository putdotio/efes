package main

import (
	"context"
	"database/sql"
	"fmt"
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
	"github.com/getsentry/raven-go"
	"github.com/shirou/gopsutil/disk"
	"github.com/streadway/amqp"
)

// Server runs on storage servers.
type Server struct {
	config               *Config
	db                   *sql.DB
	log                  log.Logger
	readServer           http.Server
	writeServer          http.Server
	amqp                 *amqpredialer.AMQPRedialer
	onceDiskStatsUpdated sync.Once
	devid                int64
	hostname             string
	shutdown             chan struct{}
	Ready                chan struct{}
	diskStatsUpdated     chan struct{}
	diskStatsStopped     chan struct{}
	diskCleanStopped     chan struct{}
	deviceCleanStopped   chan struct{}
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
	devid, err := strconv.ParseInt(strings.TrimPrefix(filepath.Base(c.Server.DataDir), "dev"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Cannot determine device ID from dir: %s", c.Server.DataDir)
	}
	db, err := openDatabase(c.Database)
	if err != nil {
		return nil, err
	}
	logger := log.NewLogger("server")
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	s := &Server{
		config:              c,
		devid:               devid,
		db:                  db,
		log:                 logger,
		hostname:            hostname,
		shutdown:            make(chan struct{}),
		Ready:               make(chan struct{}),
		diskStatsUpdated:    make(chan struct{}),
		diskStatsStopped:    make(chan struct{}),
		diskCleanStopped:    make(chan struct{}),
		deviceCleanStopped:  make(chan struct{}),
		amqpRedialerStopped: make(chan struct{}),
	}
	devicePrefix := "/" + filepath.Base(s.config.Server.DataDir)
	s.writeServer.Handler = http.StripPrefix(devicePrefix, newFileReceiver(s.config.Server.DataDir, s.log))
	s.writeServer.Handler = http.HandlerFunc(raven.RecoveryHandler(addVersion(s.writeServer.Handler)))
	s.readServer.Handler = http.StripPrefix(devicePrefix, http.FileServer(http.Dir(s.config.Server.DataDir)))
	if s.config.Debug {
		s.log.SetLevel(log.DEBUG)
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
	go s.cleanDevice()
	go s.updateDiskStats()
	go s.consumeDeleteQueue()
	s.log.Notice("Server is started.")
	errCh := make(chan error, 2)
	go func() {
		err2 := s.writeServer.Serve(writeListener)
		if err2 == http.ErrServerClosed {
			s.log.Notice("Write server is shutting down.")
		}
		errCh <- err2
	}()
	go func() {
		err2 := s.readServer.Serve(readListener)
		if err2 == http.ErrServerClosed {
			s.log.Notice("Read server is shutting down.")
		}
		errCh <- err2
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.Server.ShutdownTimeout))
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
	iostat, err := newIOStat(s.config.Server.DataDir, 10*time.Second)
	if err != nil {
		s.log.Warningln("Cannot get stats for dir:", s.config.Server.DataDir, "err:", err.Error())
	}
	for {
		select {
		case <-ticker.C:
			total, used, free := s.getDiskUsage()
			utilization := s.getDiskUtilization(iostat)
			_, err = s.db.Exec("update device set io_utilization=?, bytes_total=?, bytes_used=?, bytes_free=?, updated_at=current_timestamp where devid=?", utilization, total, used, free, s.devid)
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

func (s *Server) getDiskUsage() (total, used, free sql.NullInt64) {
	usage, err := disk.Usage(s.config.Server.DataDir)
	if err != nil {
		s.log.Errorln("Cannot get disk usage:", err.Error())
		return
	}
	total.Valid = true
	total.Int64 = int64(usage.Total)
	free.Valid = true
	free.Int64 = int64(usage.Free)
	used.Valid = true
	used.Int64 = total.Int64 - free.Int64
	return
}

func (s *Server) getDiskUtilization(iostat *IOStat) (utilization sql.NullInt64) {
	if iostat == nil {
		return
	}
	value, err := iostat.Utilization()
	if err == errUtilizationNotAvailable {
		return
	} else if err != nil {
		s.log.Errorln("Cannot get disk IO utilization:", err.Error())
		return
	}
	utilization.Valid = true
	utilization.Int64 = int64(value)
	return
}

func declareDeleteQueue(ch *amqp.Channel, devid int64) (amqp.Queue, error) {
	return ch.QueueDeclare(
		deleteQueueName(devid),
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

func deleteQueueName(devid int64) string {
	return "delete.dev" + strconv.FormatInt(devid, 10)
}

func (s *Server) consumeDeleteQueue() {
	s.log.Debug("Starting delete queue consumer..")
	for {
		select {
		case <-s.shutdown:
			return
		case conn, ok := <-s.amqp.Conn():
			if !ok {
				s.log.Error("Cannot consume delete queue. AMQP connection is not open.")
				time.Sleep(time.Second)
				continue
			}
			err := s.processDeleteTasks(conn)
			if err != nil {
				s.log.Error("Error while processing delete task", err)
				raven.CaptureError(err, nil)
			}
		}
	}
}

func (s *Server) processDeleteTasks(conn *amqp.Connection) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	q, err := declareDeleteQueue(ch, s.devid)
	if err != nil {
		return err
	}

	pid := os.Getpid()
	consumerTag := "efes-delete-worker:" + strconv.Itoa(pid) + "@" + s.hostname + "/" + strconv.FormatInt(s.devid, 10)
	messages, err := ch.Consume(
		q.Name,      // queue
		consumerTag, // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return err
	}
	for {
		select {
		case <-s.shutdown:
			return nil
		case msg, ok := <-messages:
			if !ok {
				return amqp.ErrClosed
			}
			msgBody := string(msg.Body)
			fileID, err := strconv.ParseInt(msgBody, 10, 64)
			if err != nil {
				s.log.Error("Error while parsing int", err)
				continue
			}
			err = s.deleteFidOnDisk(fileID)
			if err != nil {
				s.log.Errorf("Failed to delete fid %d, %s", fileID, err)
				err2 := msg.Nack(false, false)
				if err2 != nil {
					s.log.Errorf("NACK error: %s", err2)
					return err2
				}
				continue
			}
			err = msg.Ack(false)
			if err != nil {
				s.log.Errorf("ACK error: %s", err)
				return err
			}
		}
	}
}

func (s *Server) deleteFidOnDisk(fileID int64) error {
	s.log.Debug("Deleting fid on disk ", fileID)
	path := filepath.Join(s.config.Server.DataDir, vivify(fileID))
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.log.Debugf("Fid path does not exist %s. ", path)
			return nil
		}
		return err
	}
	s.log.Debugf("Fid %d deleted. ", fileID)
	return nil
}

func vivify(fid int64) string {
	s := fmt.Sprintf("%010d", fid)
	return fmt.Sprintf("%s/%s/%s/%s.fid", s[0:1], s[1:4], s[4:7], s)
}
