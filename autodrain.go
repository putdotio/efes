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
			manualDrain, err := s.isManualDrainEnabled()
			if err != nil {
				s.log.Errorln("Cannot determine drain status:", err)
				continue
			}
			if manualDrain {
				s.log.Info("Drain has started.")

				// No particular destination. Destination will be selected randomly.
				// Clearing it because it may be set before by auto-drain.
				d.Dest = nil

				s.drainFiles(d, s.shouldContinueManualDrain)

				s.log.Info("Drain has stopped.")
				continue
			}

			runAutoDrain, status := s.shouldRunAutoDrain()
			if !runAutoDrain {
				continue
			}
			s.log.Info("Auto-drain has started.")

			// Set drainer target to devices with usage below total average + threshold.
			// This is not need to be checked on every file moved because we don't want to create extra
			// load on tracker. Beside, it is not an information that changes frequently and auto-drainer
			// will run for a limited period of time.
			d.Dest = s.filterDevicesForAutoDrain(status)
			if len(d.Dest) == 0 {
				s.log.Warning("No devices available as auto-drain target")
				continue
			}

			s.drainFiles(d, func() bool { return s.shouldContinueAutoDrain((status.totalUse)) })

			s.log.Info("Auto-drain has stopped. Waiting for next run period.")
		case <-s.shutdown:
			return
		}
	}
}

func (s *Server) drainFiles(drainer *Drainer, condition func() bool) {
	// Keep last fid to not selecting it again.
	// Some files may not be movable due to a permission issue, etc.
	var lastFid int64

	for ok := true; ok; ok = condition() {
		fid, err := s.getNextFidForDrain(lastFid)
		if err != nil {
			s.log.Errorln("Error while getting next fid for drain operation:", err)
			raven.CaptureError(err, nil)

			// Stop draining because the error may repeat.
			// Drain will be triggered again with Ticker.
			return
		}
		lastFid = fid
		err = drainer.moveFile(fid)
		if err != nil {
			s.log.Errorln("Error while drain is moving a file:", err)
			raven.CaptureError(err, nil)

			// We can continue with the next file.
			continue
		}
	}
}

// isMyAutoDrainPeriod is here for limiting the number of parallel auto-drain operations to some portion of the cluster.
// All servers call this method at the same time to see if they are allowed to run auto-drain for current period.
// Period duration can be configured with Config.AutoDrainRunPeriod option.
// To tweak the number of devices, you can use Config.AutoDrainDeviceRatio option.
// If AutoDrainDeviceRatio is 8, only 1/8 of the total servers run auto-drain in parallel.
func (s *Server) isMyAutoDrainPeriod(now time.Time) bool {
	period := now.UTC().UnixNano() / int64(s.config.Server.AutoDrainRunPeriod)
	devid := hashDevid(s.devid)
	ratio := int64(s.config.Server.AutoDrainDeviceRatio)
	return (period+devid)%ratio == 0
}

func (s *Server) shouldRunAutoDrain() (bool, *efesStatus) {
	if !s.isMyAutoDrainPeriod(time.Now()) {
		// Not our turn, other servers should be running.
		// We will check again on next period.
		return false, nil
	}
	client, err := NewClient(s.config)
	if err != nil {
		s.log.Errorln("Error while creating client:", err)
		return false, nil
	}
	status, err := client.fetchStatus()
	if err != nil {
		s.log.Errorln("Error while getting device statuses:", err)
		return false, nil
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
		s.log.Errorln("Current device not found in tracker response")
		return false, nil
	}

	return deviceUse(currentDevice) > status.totalUse+int64(s.config.Server.AutoDrainThreshold), status
}

// shouldContinueAutoDrain is similar to shouldRunAutoDrain but,
// instead of asking tracker for disk stats, it queries the local disk to reduce load tracker.
func (s *Server) shouldContinueAutoDrain(totalUse int64) bool {
	return s.isMyAutoDrainPeriod(time.Now()) && s.getDiskUse() > totalUse+int64(s.config.Server.AutoDrainThreshold)
}

// deviceUse returns usage percentage of a device from tracker response.
func deviceUse(d deviceStatus) int64 {
	if d.BytesUsed == nil || d.BytesTotal == nil {
		return -1
	}
	return (*d.BytesUsed * 100) / *d.BytesTotal
}

// filterDevicesForAutoDrain returns devices only below the target use percentage.
// Target percentage is calculated as TotalUseOfCluster - AutoDrainThreshold.
func (s *Server) filterDevicesForAutoDrain(status *efesStatus) []int64 {
	ret := make([]int64, 0)
	target := status.totalUse - int64(s.config.Server.AutoDrainThreshold)
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

// getNextFidForDrain always selects the minimum fid number.
// After the file is moved, fid is assigned to another device so it is going to select the next one in next call.
func (s *Server) getNextFidForDrain(lastFid int64) (int64, error) {
	row := s.db.QueryRow("select min(fid) from file_on where devid=? and fid>?", s.devid, lastFid)
	var fid int64
	err := row.Scan(&fid)
	return fid, err
}

func (s *Server) isManualDrainEnabled() (bool, error) {
	row := s.db.QueryRow("select status from device where devid=?", s.devid)
	var status string
	err := row.Scan(&status)
	return status == "drain", err
}

func (s *Server) shouldContinueManualDrain() bool {
	drainEnabled, _ := s.isManualDrainEnabled()
	return drainEnabled
}
