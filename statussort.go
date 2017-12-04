package main

type deviceStatuses []deviceStatus

func (a deviceStatuses) Len() int      { return len(a) }
func (a deviceStatuses) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type byHostname struct{ deviceStatuses }

func (a byHostname) Less(i, j int) bool {
	return a.deviceStatuses[i].Hostname < a.deviceStatuses[j].Hostname
}

type byDevID struct{ deviceStatuses }

func (a byDevID) Less(i, j int) bool { return a.deviceStatuses[i].Devid < a.deviceStatuses[j].Devid }

type bySize struct{ deviceStatuses }

func (a bySize) Less(i, j int) bool {
	t1, t2 := a.deviceStatuses[i].BytesTotal, a.deviceStatuses[j].BytesTotal
	if t1 == nil || t2 == nil {
		return false
	}
	return *t1 < *t2
}

type byUsed struct{ deviceStatuses }

func (a byUsed) Less(i, j int) bool {
	t1, t2 := a.deviceStatuses[i].BytesUsed, a.deviceStatuses[j].BytesUsed
	if t1 == nil || t2 == nil {
		return false
	}
	return *t1 < *t2
}

type byFree struct{ deviceStatuses }

func (a byFree) Less(i, j int) bool {
	t1, t2 := a.deviceStatuses[i].BytesFree, a.deviceStatuses[j].BytesFree
	if t1 == nil || t2 == nil {
		return false
	}
	return *t1 < *t2
}
