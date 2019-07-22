package main

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

type efesStatus struct {
	devices    []deviceStatus
	serverTime time.Time
}

type deviceStatus struct {
	Device
	Hostname   string
	HostStatus string
	UpdatedAt  time.Time
}

func (d deviceStatus) Size() string {
	if d.BytesTotal == nil {
		return ""
	}
	return humanize.Comma(*d.BytesTotal / G)
}

func (d deviceStatus) Used() string {
	if d.BytesUsed == nil {
		return ""
	}
	return humanize.Comma(*d.BytesUsed / G)
}

func (d deviceStatus) Free() string {
	if d.BytesFree == nil {
		return ""
	}
	return humanize.Comma(*d.BytesFree / G)
}

func (d deviceStatus) Use() string {
	if d.BytesUsed == nil || d.BytesTotal == nil {
		return ""
	}
	use := (*d.BytesUsed * 100) / *d.BytesTotal
	return colorPercent(use, strconv.FormatInt(use, 10))
}

func (d deviceStatus) IO() string {
	if d.IoUtilization == nil {
		return ""
	}
	return colorPercent(*d.IoUtilization, strconv.FormatInt(*d.IoUtilization, 10))
}

func colorPercent(value int64, s string) string {
	switch {
	case value >= 90:
		return color.RedString(s)
	case value >= 80:
		return color.YellowString(s)
	case value < 10:
		return color.BlueString(s)
	}
	return s
}

func colorDuration(value time.Duration, s string) string {
	if value > 2*time.Second {
		return color.RedString(s)
	} else if value > 1*time.Second {
		return color.YellowString(s)
	}
	return s
}

func colorStatus(status string) string {
	if status == "alive" {
		return status
	} else if status == "down" {
		return color.RedString(status)
	}
	return color.YellowString(status)
}

func (s *efesStatus) Print() {
	// Sum totals
	var totalUsed, totalSize, totalFree int64 // in MB
	for _, d := range s.devices {
		if d.BytesUsed != nil {
			totalUsed += *d.BytesUsed
		}
		if d.BytesTotal != nil {
			totalSize += *d.BytesTotal
		}
		if d.BytesFree != nil && d.Status == "alive" {
			totalFree += *d.BytesFree
		}
	}
	var totalUse int64
	if totalSize == 0 {
		totalUse = 0
	} else {
		totalUse = 100 - (100*totalFree)/totalSize
	}

	// Convert to GB
	totalUsed /= G
	totalFree /= G
	totalSize /= G

	// Setup table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	table.SetHeader([]string{
		"Host",
		"Status",
		"Device",
		"Status",
		"Size (G)",
		"Used (G)",
		"Free (G)",
		"Use %",
		"IO %",
		"Last update",
	})

	// Add data to the table
	data := make([][]string, len(s.devices))
	for i, d := range s.devices {
		updatedAt := s.serverTime.Sub(d.UpdatedAt).Truncate(time.Second)
		data[i] = []string{
			d.Hostname,
			colorStatus(d.HostStatus),
			strconv.FormatInt(d.Devid, 10),
			colorStatus(d.Status),
			d.Size(),
			d.Used(),
			d.Free(),
			d.Use(),
			d.IO(),
			colorDuration(updatedAt, updatedAt.String()),
		}

	}
	table.AppendBulk(data) // Add Bulk Data

	table.SetFooter([]string{
		"", "", "",
		"Total:",
		humanize.Comma(totalSize),
		humanize.Comma(totalUsed),
		humanize.Comma(totalFree),
		strconv.FormatInt(totalUse, 10),
		"", "",
	})
	table.Render()
}

// nolint
func (c *Client) Status(sortBy string) (*efesStatus, error) {
	ret := &efesStatus{
		devices: make([]deviceStatus, 0),
	}
	var devices GetDevices
	resp, err := c.request(http.MethodGet, "get-devices", nil, &devices)
	if err != nil {
		return nil, err
	}
	ret.serverTime, err = http.ParseTime(resp.Header.Get("date"))
	if err != nil {
		ret.serverTime = time.Now()
	}
	ret.serverTime = ret.serverTime.UTC()
	var hosts GetHosts
	_, err = c.request(http.MethodGet, "get-hosts", nil, &hosts)
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
		var hostStatus string
		if h, ok := hostsByID[d.Hostid]; ok {
			hostname = h.Hostname
			hostStatus = h.Status
		}
		ds := deviceStatus{
			Device:     d,
			Hostname:   hostname,
			HostStatus: hostStatus,
			UpdatedAt:  time.Unix(d.UpdatedAt, 0),
		}
		ret.devices = append(ret.devices, ds)
	}
	switch sortBy {
	case "host":
		sort.Sort(byHostname{ret.devices})
	case "device":
		sort.Sort(byDevID{ret.devices})
	case "size":
		sort.Sort(bySize{ret.devices})
	case "used":
		sort.Sort(byUsed{ret.devices})
	case "free":
		sort.Sort(byFree{ret.devices})
	default:
		c.log.Warningln("Sort key is not valid:", sortBy)
	}
	return ret, nil
}
