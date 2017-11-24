package main

import (
	"container/list"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/disk"
)

// IOStat implements some functionality of the "iostat" command.
type IOStat struct {
	device       string
	window       time.Duration
	measurements *list.List
}

func newIOStat(path string, window time.Duration) (*IOStat, error) {
	device, err := findDevice(path)
	if err != nil {
		return nil, err
	}
	s := &IOStat{
		device:       filepath.Base(device),
		window:       window,
		measurements: list.New(),
	}
	return s, s.measure()
}

type IOMeasurement struct {
	t time.Time
	c uint64
}

func (d *IOStat) measure() error {
	r, err := disk.IOCounters()
	if err != nil {
		return err
	}
	c, ok := r[d.device]
	if !ok {
		return fmt.Errorf("Cannot find stats for device: %s", d.device)
	}
	m := IOMeasurement{
		t: time.Now(),
		c: c.IoTime,
	}
	d.measurements.PushFront(m)
	return nil
}

var errUtilizationNotAvailable = errors.New("disk utilization is not available yet")

// Utilization returns the utilization level of the disk.
func (d *IOStat) Utilization() (percent uint8, err error) {
	err = d.measure()
	if err != nil {
		return
	}
	frameEnd := d.measurements.Front().Value.(IOMeasurement)
	frameBegin := d.measurements.Back().Value.(IOMeasurement)
	expired := make([]*list.Element, 0, 1)
	for e := d.measurements.Back(); e != nil; e = e.Prev() {
		m := e.Value.(IOMeasurement)
		if frameEnd.t.Sub(m.t) >= d.window+time.Second/2 {
			expired = append(expired, e)
			continue
		}
		frameBegin = m
		break
	}
	for _, e := range expired {
		d.measurements.Remove(e)
	}
	diffTime := frameEnd.t.Sub(frameBegin.t)
	if diffTime == 0 {
		err = errUtilizationNotAvailable
		return
	}
	diffIO := time.Duration(frameEnd.c-frameBegin.c) * time.Millisecond
	percent = uint8((100 * diffIO) / diffTime)
	return
}

func findDevice(path string) (string, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	partitions, err := disk.Partitions(true)
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
