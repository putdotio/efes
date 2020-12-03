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
	Dest     []int64
	config   *Config
	devid    int64
	db       *sql.DB
	client   *Client
	log      log.Logger
	shutdown chan struct{}
	stopped  chan struct{}

	stopOnError bool
}

func NewDrainer(c *Config) (*Drainer, error) {
	fi, err := os.Stat(c.Server.DataDir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("path must be a directory: %s", c.Server.DataDir)
	}
	devid, err := strconv.ParseInt(strings.TrimPrefix(filepath.Base(c.Server.DataDir), "dev"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("cannot determine device ID from dir: %s", c.Server.DataDir)
	}
	db, err := openDatabase(c.Database)
	if err != nil {
		return nil, err
	}
	clt, err := NewClient(c)
	if err != nil {
		return nil, err
	}
	clt.drainer = true
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
	defer rows.Close()
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
			d.log.Error(err)
			if d.stopOnError {
				return err
			}
		}
	}
	return nil
}

func (d *Drainer) moveFile(fid int64) error {
	fidpath := filepath.Join(d.config.Server.DataDir, vivify(fid))
	f, err := os.Open(fidpath)
	if os.IsNotExist(err) {
		d.log.Warningf("file (%s) does not exist on disk; removing fid (%d) from device (%d)", fidpath, fid, d.devid)
		_, err = d.db.Exec("delete from file_on where fid=? and devid=?", fid, d.devid)
		return err
	}
	if err != nil {
		return err
	}
	defer logCloseFile(d.log, f)
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	ad, err := findAliveDevice(d.db, fi.Size(), d.Dest, d.devid, "")
	if err != nil {
		return err
	}
	newPath := ad.PatchURL(fid)
	_, err = d.client.sendFile(newPath, f, fi.Size())
	if err != nil {
		return err
	}
	_, err = d.db.Exec("update file_on set devid=? where devid=? and fid=?", ad.devid, d.devid, fid)
	if err != nil {
		return err
	}
	return os.Remove(fidpath)
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
