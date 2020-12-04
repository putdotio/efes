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

	// Drainer is written for "efes drain" command but can be reused here because we need same functionality here.
	d, err := NewDrainer(s.config)
	if err != nil {
		s.log.Errorln("Error while initializing auto-drain:", err)
		raven.CaptureError(err, nil)
		return
	}
	defer d.Shutdown() // nolint:errcheck

	// Check to see if auto-drain needs to be started every minute.
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !s.shouldRunAutoDrain(time.Now()) {
				continue
			}
			s.log.Info("Auto-drain has started.")

			// Get usage infa of all devices and total use percent.
			status, err := d.client.fetchStatus()
			if err != nil {
				s.log.Errorln("Error while getting device statuses:", err)
				raven.CaptureError(err, nil)
				continue
			}

			// Set drainer target to devices with usage below total average + threshold.
			// This is not need to be checked on every file moved because we don't want to create extra
			// load on tracker. Beside, it is not an information that changes frequently and auto-drainer
			// will run for a limited period of time.
			d.Dest = s.filterDevicesForAutoDrain(status)

			for ok := true; ok; ok = s.shouldRunAutoDrain(time.Now()) {
				fid, err := s.autoDrainGetNextFid()
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

func (s *Server) shouldRunAutoDrain(now time.Time) bool {
	period := now.UTC().UnixNano() / int64(s.config.Server.AutoDrainRunPeriod)
	devid := hashDevid(s.devid)
	ratio := int64(s.config.Server.AutoDrainDeviceRatio)
	if (period+devid)%ratio != 0 {
		// Not our turn, other servers should be running.
		// We will check again on next period.
		return false
	}
	client, err := NewClient(s.config)
	if err != nil {
		s.log.Errorln("Error while creating client:", err)
		return false
	}
	status, err := client.fetchStatus()
	if err != nil {
		s.log.Errorln("Error while getting device statuses:", err)
		return false
	}

	// Find usage info of current device from tracker response.
	var currentDevice deviceStatus
	for _, dev := range status.devices {
		if dev.Devid == s.devid {
			currentDevice = dev
			break
		}
	}

	if currentDevice.Devid == 0 {
		// Cannot find current device in tracker response.
		// This cannot happen but let's be cautious.
		return false
	}

	return deviceUse(currentDevice) > status.totalUse+int64(s.config.Server.AutoDrainThreshold)
}

// deviceUse returns usage percentage of a device from tracker response.
func deviceUse(d deviceStatus) int64 {
	if d.BytesUsed == nil || d.BytesTotal == nil {
		return -1
	}
	return (*d.BytesUsed * 100) / *d.BytesTotal
}

func (s *Server) filterDevicesForAutoDrain(status *efesStatus) []int64 {
	ret := make([]int64, 0)
	target := status.totalUse + int64(s.config.Server.AutoDrainThreshold)
	for _, d := range status.devices {
		if d.Status != "alive" { // nolint:goconst
			continue
		}
		if d.BytesUsed == nil || d.BytesTotal == nil {
			continue
		}
		use := deviceUse(d)
		if use == -1 {
			continue
		}
		if use < target {
			ret = append(ret, d.Devid)
		}
	}
	return ret
}

// hashDevid takes an int64 devid and returns another integer that is randomly distributed over int64 space.
// This is for making period calculation independent of devid.
// A rare case but suppose all your devids are multiples of 10 and Config.AutoDrainDeviceRatio is also 10.
// In this case all of your devices start auto-drain in same period. We don't want that.
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

// autoDrainGetNextFid always selects the minimum fid number.
// After the file is moved, fid is assigned to another device so it is going to select the next one in next call.
func (s *Server) autoDrainGetNextFid() (int64, error) {
	row := s.db.QueryRow("select min(fid) from file_on where devid=?", s.devid)
	var fid int64
	err := row.Scan(&fid)
	return fid, err
}