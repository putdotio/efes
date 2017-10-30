package server

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/disk"
)

// IOStat implements some functionality of the "iostat" command.
type IOStat struct {
	device string
	ioTime uint64
	t      time.Time
}

func newIOStat(path string) (*IOStat, error) {
	device, err := findDevice(path)
	if err != nil {
		return nil, err
	}
	return &IOStat{
		device: device,
	}, nil
}

var errFirstRun = errors.New("first utilization call")

// Utilization returns the utilization level of the disk.
func (d *IOStat) Utilization() (percent uint8, err error) {
	now := time.Now()
	r, err := disk.IOCounters()
	if err != nil {
		return
	}
	c, ok := r[filepath.Base(d.device)]
	if !ok {
		err = fmt.Errorf("Cannot find stats for device: %s", d.device)
		return
	}
	if d.t.IsZero() {
		d.ioTime = c.IoTime
		d.t = now
		err = errFirstRun
		return
	}
	diffIO := time.Duration(c.IoTime-d.ioTime) * time.Millisecond
	d.ioTime = c.IoTime

	diffTime := now.Sub(d.t)
	d.t = now

	percent = uint8((100 * diffIO) / diffTime)
	return
}

func findDevice(path string) (string, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	partitions, err := disk.Partitions(false)
	if err != nil {
		return "", err
	}
	mountPoints := make([]string, len(partitions))
	for i, p := range partitions {
		mountPoints[i] = p.Mountpoint
	}
	i := findLongestPrefix(realPath, mountPoints)
	if i == -1 {
		return "", fmt.Errorf("Cannot find mountpoint for path: %s", path)
	}
	return partitions[i].Device, nil
}

func findLongestPrefix(s string, prefixes []string) int {
	found := -1
	lenFound := 0
	for i, pfx := range prefixes {
		if !strings.HasPrefix(s, pfx) {
			continue
		}
		if len(pfx) <= lenFound {
			continue
		}
		found = i
		lenFound = len(pfx)
	}
	return found
}
