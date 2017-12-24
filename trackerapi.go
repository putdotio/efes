package main

type GetPaths struct {
	Paths []string `json:"paths"`
}

type GetPath struct {
	Path string `json:"path"`
}

type CreateOpen struct {
	Path string `json:"path"`
	Fid  int64  `json:"fid"`
}

type GetDevices struct {
	Devices []Device `json:"devices"`
}

type Device struct {
	Devid         int64  `json:"devid"`
	Hostid        int64  `json:"hostid"`
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
