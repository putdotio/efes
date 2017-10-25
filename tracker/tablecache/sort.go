package tablecache

import (
	"errors"
	"math/rand"
	"sort"

	"github.com/putdotio/efes/constants"
)

var (
	freeSpaceFactor = 1
	ioUtilFactor    = 5
)

type device struct {
	id        int
	ioUtil    int
	freeSpace int
}

// Len returns item count.
func (d Devices) Len() int {
	return len(d.devices)
}

// Swap swaps items in heap.
func (d Devices) Swap(i, j int) {
	d.devices[i], d.devices[j] = d.devices[j], d.devices[i]
}

func (d Devices) utilization(idx int) int {
	freeUtil := d.devices[idx].freeSpace * d.freeSpaceFactor
	ioUtil := (100 - d.devices[idx].ioUtil) * d.ioUtilFactor
	return ioUtil + freeUtil
}

// Less compares items in heap.
func (d Devices) Less(i, j int) bool {
	return d.utilization(i) > d.utilization(j)
}

// Devices is a struct that implements sort.Interface.
type Devices struct {
	freeSpaceFactor int
	ioUtilFactor    int
	devices         []device
}

func isReadable(status string) bool {
	return status == constants.DeviceStatusAlive || status == constants.DeviceStatusReadOnly ||
		status == constants.DeviceStatusDrain || status == constants.DeviceStatusEmpty
}

func (c *TableCache) collectDeviceData(deviceIDs []int, readRequest bool) Devices {
	c.RLock()
	defer c.RUnlock()

	if readRequest {
		freeSpaceFactor = 0
	}
	h := Devices{
		freeSpaceFactor: freeSpaceFactor,
		ioUtilFactor:    ioUtilFactor,
		devices:         []device{},
	}
	for _, devID := range deviceIDs {
		if dev, ok := c.DeviceByID[devID]; ok {
			if !readRequest && dev.Status != constants.DeviceStatusAlive {
				continue
			}
			if readRequest && !isReadable(dev.Status) {
				continue
			}
			var fs int
			if dev.TotalSpace != 0 {
				fs = int(dev.FreeSpace * 100 / dev.TotalSpace)
			}
			d := device{
				id:        devID,
				freeSpace: fs,
				ioUtil:    dev.IOUtilization,
			}
			h.devices = append(h.devices, d)
		}
	}
	return h
}

// SortDeviceIDsByUtilization sorts given deviceIDs by utilization levels.
func (c *TableCache) SortDeviceIDsByUtilization(deviceIDs []int, readRequest bool) []int {
	devs := c.collectDeviceData(deviceIDs, readRequest)
	sort.Sort(devs)
	res := []int{}
	for _, dev := range devs.devices {
		res = append(res, dev.id)
	}
	return res
}

// WeightedChoice returns a deviceID among under-utilized devices.
func (c *TableCache) WeightedChoice(deviceIDs []int, readRequest bool) (int, error) {
	devs := c.collectDeviceData(deviceIDs, readRequest)
	sort.Sort(devs)

	l := devs.Len()
	if l == 1 {
		return devs.devices[0].id, nil
	}
	half := devs.devices[:l/2]
	total := 0
	for i := range half {
		total += devs.utilization(i)
	}
	r := rand.Intn(total)
	upTo := 0
	for i, dev := range half {
		w := devs.utilization(i)
		if upTo+w >= r {
			return dev.id, nil
		}
		upTo += w
	}
	return 0, errors.New("No device found to choice")
}
