package main

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
)

type efesStatus struct {
	devices []deviceStatus
}

type deviceStatus struct {
	Device
	Hostname  string
	UpdatedAt time.Time
}

func (d deviceStatus) Size() string {
	if d.MbTotal == nil {
		return ""
	}
	return humanize.Comma(*d.MbTotal / 1024)
}

func (d deviceStatus) Used() string {
	if d.MbUsed == nil {
		return ""
	}
	return humanize.Comma(*d.MbUsed / 1024)
}

func (d deviceStatus) Free() string {
	if d.MbUsed == nil || d.MbTotal == nil {
		return ""
	}
	free := *d.MbTotal - *d.MbUsed
	return humanize.Comma(free / 1024)
}

func (d deviceStatus) Use() string {
	if d.MbUsed == nil || d.MbTotal == nil {
		return ""
	}
	use := (*d.MbUsed * 100) / *d.MbTotal
	return strconv.FormatInt(use, 10)
}

func (d deviceStatus) IO() string {
	if d.IoUtilization == nil {
		return ""
	}
	return strconv.FormatInt(*d.IoUtilization, 10)
}

func (s *efesStatus) Print() {
	// Sum totals
	var totalUsed, totalSize int64 // in MB
	for _, d := range s.devices {
		if d.MbUsed != nil {
			totalUsed += *d.MbUsed
		}
		if d.MbTotal != nil {
			totalSize += *d.MbTotal
		}
	}
	totalFree := totalSize - totalUsed
	totalUse := (100 * totalUsed) / totalSize

	// Convert to GB
	totalUsed /= 1024
	totalFree /= 1024
	totalSize /= 1024

	// Setup table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	table.SetHeader([]string{
		"Host",
		"Device",
		"Status",
		"Size (G)",
		"Used (G)",
		"Free (G)",
		"Use %",
		"IO %",
		"Last update",
	})
	table.SetFooter([]string{
		"", "",
		"Total:",
		humanize.Comma(totalSize),
		humanize.Comma(totalUsed),
		humanize.Comma(totalFree),
		strconv.FormatInt(totalUse, 10),
		"", "",
	})

	// Add data to the table
	now := time.Now().UTC()
	data := make([][]string, len(s.devices))
	for i, d := range s.devices {
		data[i] = []string{
			d.Hostname,
			strconv.FormatInt(d.Devid, 10),
			d.Status,
			d.Size(),
			d.Used(),
			d.Free(),
			d.Use(),
			d.IO(),
			now.Sub(d.UpdatedAt).Truncate(time.Second).String(),
		}

	}
	table.AppendBulk(data) // Add Bulk Data
	table.Render()
}

func (c *Client) Status() (*efesStatus, error) {
	ret := &efesStatus{
		devices: make([]deviceStatus, 0),
	}
	var devices GetDevices
	err := c.request(http.MethodGet, "get-devices", nil, &devices)
	if err != nil {
		return nil, err
	}
	var hosts GetHosts
	err = c.request(http.MethodGet, "get-hosts", nil, &hosts)
	if err != nil {
		return nil, err
	}
	hostsByID := make(map[int64]Host)
	for _, h := range hosts.Hosts {
		hostsByID[h.Hostid] = h
	}
	for _, d := range devices.Devices {
		if d.Status == "dead" {
			continue
		}
		var hostname string
		if h, ok := hostsByID[d.Hostid]; ok {
			hostname = h.Hostname
		}
		ds := deviceStatus{
			Device:    d,
			Hostname:  hostname,
			UpdatedAt: time.Unix(d.UpdatedAt, 0),
		}
		ret.devices = append(ret.devices, ds)
	}
	return ret, nil
}
