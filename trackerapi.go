package main

type GetPath struct {
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
}

type GetPaths struct {
	Paths []GetPath `json:"paths"`
}

type CreateOpen struct {
	Path string `json:"path"`
	Fid  int64  `json:"fid"`
}

type CreateClose struct {
	Path string `json:"path"`
}

type GetDevices struct {
	Devices []Device `json:"devices"`
}

type Device struct {
	Devid         int64  `json:"devid"`
	Hostid        int64  `json:"hostid"`
	HostName      string `json:"host_name"`
	HostStatus    string `json:"host_status"`
	Rackid        int64  `json:"rackid"`
	RackName      string `json:"rack_name"`
	Zoneid        int64  `json:"zoneid"`
	ZoneName      string `json:"zone_name"`
	Status        string `json:"status"`
	BytesTotal    *int64 `json:"bytes_total"`
	BytesUsed     *int64 `json:"bytes_used"`
	BytesFree     *int64 `json:"bytes_free"`
	UpdatedAt     int64  `json:"updated_at"`
	IoUtilization *int64 `json:"io_utilization"`
}

type GetHosts struct {
	Hosts []Host `json:"hosts"`
}

type Host struct {
	Hostid   int64  `json:"hostid"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
	HostIP   string `json:"hostip"`
}

type Rack struct {
	Rackid int64  `json:"rackid"`
	Zoneid int64  `json:"zoneid"`
	Name   string `json:"name"`
}

type GetRacks struct {
	Racks []Rack `json:"racks"`
}

type Zone struct {
	Zoneid int64  `json:"zoneid"`
	Name   string `json:"name"`
}

type GetZones struct {
	Zones []Zone `json:"zones"`
}
