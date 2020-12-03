package main

// nolint:gosec
import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"time"

	"github.com/getsentry/raven-go"
)

func (s *Server) autoDrain() {
	defer close(s.autoDrainStopped)

	d, err := NewDrainer(s.config)
	if err != nil {
		s.log.Errorln("Error while initializing auto-drain:", err)
		raven.CaptureError(err, nil)
		return
	}
	defer d.Shutdown() // nolint:errcheck

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !s.shouldRunAutoDrain(time.Now()) {
				continue
			}
			s.log.Info("Auto-drain has started.")
			var lastFid int64
			status, err := d.client.fetchStatus()
			if err != nil {
				s.log.Errorln("Error while getting device statuses:", err)
				raven.CaptureError(err, nil)
				continue
			}
			d.Dest = s.filterDevicesForAutoDrain(status)
			for ok := true; ok; ok = s.shouldRunAutoDrain(time.Now()) {
				fid, err := s.autoDrainGetNextFid(lastFid)
				if err != nil {
					s.log.Errorln("Error while getting next fid for auto-drain operation:", err)
					raven.CaptureError(err, nil)
					continue
				}
				err = d.moveFile(fid)
				if err != nil {
					s.log.Errorln("Error while auto-drain is moving a file:", err)
					raven.CaptureError(err, nil)
					continue
				}
			}
			s.log.Info("Auto-drain has stopped. Waiting for next run period.")
		case <-s.shutdown:
			return
		}
	}
}

func (s *Server) filterDevicesForAutoDrain(status *efesStatus) []int64 {
	ret := make([]int64, 0)
	below := status.totalUse + int64(s.config.Server.AutoDrainThreshold)
	for _, d := range status.devices {
		if d.BytesUsed == nil || d.BytesTotal == nil {
			continue
		}
		use := (*d.BytesUsed * 100) / *d.BytesTotal
		if use < below {
			ret = append(ret, d.Devid)
		}
	}
	return ret
}

func (s *Server) shouldRunAutoDrain(now time.Time) bool {
	period := now.UTC().UnixNano() / int64(s.config.Server.AutoDrainRunPeriod)
	devid := hashDevid(s.devid)
	ratio := int64(s.config.Server.AutoDrainDeviceRatio)
	return (period+devid)%ratio == 0
}

func hashDevid(devid int64) int64 {
	buf := make([]byte, 8)
	err := binary.Write(bytes.NewBuffer(buf), binary.BigEndian, devid)
	if err != nil {
		panic(err)
	}
	sum := md5.Sum(buf) // nolint:gosec
	buf = sum[:8]
	err = binary.Read(bytes.NewReader(buf), binary.BigEndian, &devid)
	if err != nil {
		panic(err)
	}
	return devid
}

func (s *Server) autoDrainGetNextFid(lastFid int64) (int64, error) {
	row := s.db.QueryRow("select fid from file_on where devid=? and fid>? order by fid asc", s.devid, lastFid)
	var fid int64
	err := row.Scan(&fid)
	return fid, err
}
