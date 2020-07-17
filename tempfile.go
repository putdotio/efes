package main

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/raven-go"
)

type Tempfile struct {
	fid, devid int64
}

func (t *Tracker) tempfileCleaner() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := t.removeOldTempfiles()
			if err != nil {
				t.log.Errorln("cannot delete old tempfile records:", err.Error())
				raven.CaptureError(err, nil)
			}
		case <-t.shutdown:
			close(t.tempfileCleanerStopped)
			return
		}
	}
}

func (t *Tracker) removeOldTempfiles() error {
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	tempfiles, err := t.removeOldTempfilesFromDB(tx)
	if err != nil {
		logRollbackTx(t.log, tx)
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	for _, tf := range tempfiles {
		go t.publishDeleteTask([]int64{tf.devid}, tf.fid)
	}
	t.log.Infoln(len(tempfiles), "old tempfile records are deleted")
	return nil
}

func (t *Tracker) removeOldTempfilesFromDB(tx *sql.Tx) (tempfiles []Tempfile, err error) {
	tempfileTooOld := time.Duration(t.config.Tracker.TempfileTooOld) / time.Microsecond
	rows, err := tx.Query("select fid, devid from tempfile where created_at < CURRENT_TIMESTAMP - INTERVAL ? MICROSECOND for update", tempfileTooOld)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tf Tempfile
		err = rows.Scan(&tf.fid, &tf.devid)
		if err != nil {
			return
		}
		tempfiles = append(tempfiles, tf)
	}
	err = rows.Err()
	if err != nil {
		return
	}
	if len(tempfiles) == 0 {
		t.log.Debug("no stale tempfile found")
		return
	}
	fids := make([]string, len(tempfiles))
	for i, tf := range tempfiles {
		fids[i] = strconv.FormatInt(tf.fid, 10)
	}
	_, err = tx.Exec("delete from tempfile where fid in (" + strings.Join(fids, ",") + ")")
	return
}
