package main

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

func (s *Server) cleanDisk() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	period := time.Duration(s.config.Server.CleanDiskRunPeriod) / time.Second
	for {
		select {
		case <-ticker.C:
			res, err := s.db.Exec("update device set last_disk_clean_time=current_timestamp where devid=? and ADDDATE(last_disk_clean_time, INTERVAL ? SECOND) < CURRENT_TIMESTAMP", s.devid, period)
			if err != nil {
				s.log.Errorln("Error during updating last disk clean time:", err)
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
			s.log.Info("Cleanup has started on data directory.")
			err = filepath.Walk(s.config.Server.DataDir, s.visitFile)
			if err != nil {
				s.log.Errorln("Error in data directory cleanup:", err)
				sentry.CaptureException(err)
			} else {
				s.log.Infoln("Data directory cleanup has finished successfully.")
			}
			// Updating last_disk_clean_time at the end of traversal helps to
			// spread the load on database more uniform in time.
			_, err = s.db.Exec("update device set last_disk_clean_time=current_timestamp where devid=?", s.devid)
			if err != nil {
				s.log.Errorln("Error during updating last disk clean time:", err)
				continue
			}
		case <-s.shutdown:
			close(s.diskCleanStopped)
			return
		}
	}
}

func (s *Server) visitFile(path string, f os.FileInfo, err error) error {
	select {
	case <-s.shutdown:
		return io.EOF
	default:
	}
	if err != nil {
		s.log.Errorln("Error while walking data dir:", err.Error())
		return nil
	}
	if f.IsDir() {
		return nil
	}
	if f.Mode()&os.ModeSymlink == os.ModeSymlink {
		return nil
	}
	ttl := time.Duration(s.config.Server.CleanDiskFileTTL)
	if time.Since(f.ModTime()) < ttl {
		s.log.Debugln("File is newer than", ttl)
		return nil
	}
	ext := filepath.Ext(path)
	if ext != ".fid" && ext != ".info" {
		s.log.Infoln("extension is not \".fid\" or \".info\":", path, "; removing...")
		err = os.Remove(path)
		if err != nil {
			s.log.Errorln("Cannot remove file:", err.Error())
		}
		return nil
	}
	// Example file name: 0000000789.fid
	fileName := strings.Split(f.Name(), ".")
	fileID, err := strconv.ParseInt(strings.TrimLeft(fileName[0], "0,"), 10, 64)
	if err != nil {
		s.log.Error("Can not parse file name ", err)
		return nil
	}
	existsOnDB, err := s.fidExistsOnDatabase(fileID)
	if err != nil {
		s.log.Errorln("Cannot query database:", err)
		return nil
	}
	if existsOnDB {
		return nil
	}
	s.log.Infof("Fid %d is too old and there is no record on DB for it. Deleting...", fileID)
	err = os.Remove(path)
	if err != nil {
		s.log.Errorln("Cannot remove file:", err.Error())
	}
	return nil
}

func (s *Server) fidExistsOnDatabase(fileID int64) (bool, error) {
	var fid int
	switch err := s.db.QueryRow("select fid from file_on where fid=? and devid=?", fileID, s.devid).Scan(&fid); err {
	case sql.ErrNoRows:
		switch err = s.db.QueryRow("select fid from tempfile where fid=? and devid=?", fileID, s.devid).Scan(&fid); err {
		case sql.ErrNoRows:
			return false, nil
		case nil:
			return true, nil
		default:
			return true, err
		}
	case nil:
		return true, nil
	default:
		return true, err
	}
}
