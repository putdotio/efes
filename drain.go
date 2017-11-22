package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cenkalti/log"
)

type Drainer struct {
	config   *Config
	checksum bool
	devid    uint64
	db       *sql.DB
	client   *Client
	log      log.Logger
	shutdown chan struct{}
	stopped  chan struct{}
}

func NewDrainer(c *Config) (*Drainer, error) {
	fi, err := os.Stat(c.Server.DataDir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("Path must be a directory: %s", c.Server.DataDir)
	}
	devid, err := strconv.ParseUint(strings.TrimPrefix(filepath.Base(c.Server.DataDir), "dev"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Cannot determine device ID from dir: %s", c.Server.DataDir)
	}
	db, err := sql.Open("mysql", c.Database.DSN)
	if err != nil {
		return nil, err
	}
	clt, err := NewClient(c)
	if err != nil {
		return nil, err
	}
	logger := log.NewLogger("drain")
	d := &Drainer{
		config:   c,
		devid:    devid,
		db:       db,
		client:   clt,
		log:      logger,
		shutdown: make(chan struct{}),
		stopped:  make(chan struct{}),
	}
	if d.config.Debug {
		d.log.SetLevel(log.DEBUG)
	}
	return d, nil
}

func (d *Drainer) Run() error {
	d.log.Noticeln("Setting device status to 'drain' on device:", d.devid)
	_, err := d.db.Exec("update device set status='drain' where devid=?", d.devid)
	if err != nil {
		return err
	}
	var fids []int64
	rows, err := d.db.Query("select fid from file_on where devid=?", d.devid)
	if err != nil {
		return err
	}
	defer rows.Close() // nolint: errcheck
	for rows.Next() {
		var fid int64
		err = rows.Scan(&fid)
		if err != nil {
			return err
		}
		fids = append(fids, fid)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for i, fid := range fids {
		select {
		case <-d.shutdown:
			close(d.stopped)
			return nil
		default:
		}
		d.log.Infof("moving fid=%v; %v of %v (%v%%)", fid, i+1, len(fids), ((i+1)*100)/len(fids))
		if err = d.moveFile(fid); err != nil {
			if os.IsNotExist(err) {
				d.log.Error(err)
				continue
			}
			return err
		}
	}
	return nil
}

func (d *Drainer) moveFile(fid int64) error {
	sfid := fmt.Sprintf("%010d", fid)
	fidpath := fmt.Sprintf("/%s/%s/%s/%s.fid", sfid[0:1], sfid[1:4], sfid[4:7], sfid)
	fidpath = filepath.Join(d.config.Server.DataDir, fidpath)

	f, err := os.Open(fidpath)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	ad, err := findAliveDevice(d.db, fi.Size())
	if err != nil {
		return err
	}
	newPath := ad.PatchURL(fid)
	_, err = d.client.sendFile(newPath, f, fi.Size())
	if err != nil {
		return err
	}
	if d.checksum {
		var remoteChecksum string
		remoteChecksumCalculated := make(chan struct{})
		go func() {
			defer func() { close(remoteChecksumCalculated) }()
			var errRemote error
			remoteChecksum, errRemote = d.client.crc32(newPath)
			if errRemote != nil {
				d.log.Errorln("cannot calculate remote crc32:", errRemote)
				return
			}
		}()
		localChecksum, err := crc32file(fidpath)
		if err != nil {
			return err
		}
		<-remoteChecksumCalculated
		if remoteChecksum != localChecksum {
			return fmt.Errorf("crc32 mismatch: local=%s, remote=%s", localChecksum, remoteChecksum)
		}
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec("insert into file_on(fid, devid) values(?, ?)", fid, ad.devid)
	if err != nil {
		return err
	}
	_, err = tx.Exec("delete from file_on where fid=? and devid=?", fid, d.devid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Drainer) Shutdown() error {
	close(d.shutdown)
	<-d.stopped

	err := d.db.Close()
	if err != nil {
		d.log.Error("Error while closing database connection")
		return err
	}
	return nil
}
