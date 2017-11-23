package main

type GetPaths struct {
	Paths []string `json:"paths"`
}

type CreateOpen struct {
	Path  string `json:"path"`
	Fid   int64  `json:"fid"`
	Devid int64  `json:"devid"`
}

type GetDevices struct {
	Devices []Device `json:"devices"`
}

type Device struct {
	Devid         int64  `json:"devid"`
	Hostid        int64  `json:"hostid"`
	Status        string `json:"status"`
	MbTotal       *int64 `json:"mb_total"`
	MbUsed        *int64 `json:"mb_used"`
	UpdatedAt     int64  `json:"mb_asof"`
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
