package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

func (s *Server) cleanDevice() {
	s.log.Notice("Starting device cleaner...")
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	period := time.Duration(s.config.Server.CleanDeviceRunPeriod) / time.Second
	for {
		select {
		case <-ticker.C:
			res, err := s.db.Exec("update device "+
				"set last_device_clean_time=current_timestamp "+
				"where devid=? "+
				"and ADDDATE(last_device_clean_time, INTERVAL ? SECOND) < CURRENT_TIMESTAMP",
				s.devid, period)
			if err != nil {
				s.log.Errorln("Error during updating last device clean time:", err)
				continue
			}
			ra, err := res.RowsAffected()
			if err != nil {
				s.log.Errorln("Cannot get rows affected:", err)
				continue
			}
			if ra == 0 {
				continue
			}
			s.log.Info("Cleanup has started on database table.")
			err = s.walkOnDeviceFiles()
			if err != nil {
				s.log.Errorln("Error in database table cleanup:", err)
				sentry.CaptureException(err)
			} else {
				s.log.Info("Database table cleanup has finished successfully.")
			}
			// Updating last_device_clean_time at the end of traversal helps to
			// spread the load on database more uniform in time.
			_, err = s.db.Exec("update device set last_device_clean_time=current_timestamp where devid=?", s.devid)
			if err != nil {
				s.log.Errorln("Error during updating last device clean time:", err)
				continue
			}
		case <-s.shutdown:
			close(s.deviceCleanStopped)
			return
		}
	}
}

func (s *Server) walkOnDeviceFiles() error {
	fids, err := s.getDeviceFids()
	if err != nil {
		return err
	}
	for _, fid := range fids {
		err := s.checkFid(fid)
		if err != nil {
			s.log.Errorf("cannot check fid [%d]: %s", fid, err.Error())
		}
	}
	return nil
}

func (s *Server) getDeviceFids() (fids []int64, err error) {
	rows, err := s.db.Query("select fid from file_on where devid=?", s.devid)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var fid int64
		err = rows.Scan(&fid)
		if err != nil {
			return
		}
		fids = append(fids, fid)
	}
	err = rows.Err()
	return
}

func (s *Server) checkFid(fid int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // nolint: errcheck

	devids, err := s.getAllDevidsForFid(tx, fid)
	if err != nil {
		return err
	}
	if !inList(s.devid, devids) {
		return nil
	}
	path := filepath.Join(s.config.Server.DataDir, vivify(fid))
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		// delete from file_on for current devid
		s.log.Warningf("Deleting fid [%d] from current device", fid)
		return s.deleteFidFromCurrentDevice(tx, fid)
	} else if err != nil {
		return err
	}
	// file exist on current disk, check other disks
	otherDevids := removeItem(s.devid, devids)
	if len(otherDevids) == 0 {
		return tx.Commit()
	}
	// remove fid from other disks
	return s.deleteFidFromOtherDevices(tx, otherDevids, fid)
}

func (s *Server) deleteFidFromOtherDevices(tx *sql.Tx, otherDevids []int64, fid int64) error {
	if s.config.Server.CleanDeviceDryRun {
		s.log.Infof("Dry run: deleting fid [%d] from other devices: %v", fid, otherDevids)
		return nil
	}
	s.log.Warningf("Deleting fid [%d] from other devices: %v", fid, otherDevids)
	otherDevidsString := make([]string, len(otherDevids))
	for i, devid := range otherDevids {
		otherDevidsString[i] = strconv.FormatInt(devid, 10)
	}
	_, err := tx.Exec("delete from file_on where fid=? and devid in ("+strings.Join(otherDevidsString, ",")+")", fid)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	go s.publishDeleteTask(otherDevids, fid)
	return nil
}

func (s *Server) deleteFidFromCurrentDevice(tx *sql.Tx, fid int64) error {
	if s.config.Server.CleanDeviceDryRun {
		s.log.Infof("Dry run: deleting fid [%d] from current device", fid)
		return nil
	}
	_, err := tx.Exec("delete from file_on where fid=? and devid=?", fid, s.devid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func inList(value int64, list []int64) bool {
	found := false
	for _, current := range list {
		if current == value {
			found = true
			break
		}
	}
	return found
}

func removeItem(value int64, list []int64) []int64 {
	for i, current := range list {
		if current == value {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func (s *Server) getAllDevidsForFid(tx *sql.Tx, fid int64) (devids []int64, err error) {
	rows, err := tx.Query("select devid from file_on where fid=? for update", fid)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var devid int64
		err = rows.Scan(&devid)
		if err != nil {
			return
		}
		devids = append(devids, devid)
	}
	err = rows.Err()
	return
}

func (s *Server) publishDeleteTask(devids []int64, fid int64) {
	select {
	case conn, ok := <-s.amqp.Conn():
		if !ok {
			s.log.Error("Cannot publish delete task. AMQP connection is closed.")
			return
		}
		ch, err := conn.Channel()
		if err != nil {
			s.log.Errorln("cannot open amqp channel:", err.Error())
			return
		}
		for _, devid := range devids {
			err = publishDeleteTask(ch, devid, fid)
			if err != nil {
				s.log.Errorln("cannot publish delete task:", err.Error())
			}
		}
		err = ch.Close()
		if err != nil {
			s.log.Errorln("cannot close amqp channel:", err.Error())
		}
	case <-s.shutdown:
		s.log.Warningf("Not sending delete task for fid=%d because shutdown is requested while waiting for amqp connection", fid)
	}
}
