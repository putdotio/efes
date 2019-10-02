package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPing(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("GET", "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := "pong"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func insertHost(t *testing.T, tr *Tracker) {
	t.Helper()

	_, err := tr.db.Exec("insert into zone(zoneid, name) values(1, 'zone1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into rack(rackid, zoneid, name) values(1, 1, 'rack1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip, rackid) values(1, 'foo', 'alive', '1.2.3.4', 1)")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetPath(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	insertHost(t, tr)
	_, err = tr.db.Exec("insert into device(devid, status, hostid, read_port) values(2, 'alive', 1, 1234)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file(fid, dkey) values(42, 'foo')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file_on(fid, devid) values(42, 2)")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("GET", "/get-path?key=foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	var resp GetPath
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	expected := "http://foo:1234/dev2/0/000/000/0000000042.fid"
	if resp.Path != expected {
		t.Errorf("handler returned unexpected path: got %v want %v",
			resp.Path, expected)
	}
}

func TestCreateOpen(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	insertHost(t, tr)
	_, err = tr.db.Exec("insert into device(devid, status, hostid, bytes_total, bytes_used, bytes_free, write_port) values(2, 'alive', 1, 1000, 500, 500, 1234)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("alter table tempfile auto_increment = 5")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/create-open", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := "{\"path\":\"http://foo:1234/dev2/0/000/000/0000000005.fid\",\"fid\":5}\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestCreateOpenSameZone(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into zone(zoneid, name) values(1, 'zone1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into zone(zoneid, name) values(2, 'zone2')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into rack(rackid, zoneid, name) values(1, 1, 'rack1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into rack(rackid, zoneid, name) values(2, 1, 'rack2')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into rack(rackid, zoneid, name) values(3, 2, 'rack3')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into rack(rackid, zoneid, name) values(4, 2, 'rack4')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into subnet(subnetid, rackid, subnet) values(1, 3, '1.0.0.0/8')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip, rackid) values(1, 'foo', 'alive', '1.1.1.1', 3)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip, rackid) values(2, 'bar', 'alive', '2.2.2.2', 4)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, bytes_total, bytes_used, bytes_free, write_port) values(1, 'alive', 1, 1000, 1000, 0, 1234)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, bytes_total, bytes_used, bytes_free, write_port) values(2, 'alive', 2, 1000, 500, 500, 1234)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("alter table tempfile auto_increment = 5")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/create-open?size=100", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-forwarded-for", "1.1.1.1")
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := "{\"path\":\"http://bar:1234/dev2/0/000/000/0000000005.fid\",\"fid\":5}\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestCreateClose(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	insertHost(t, tr)
	_, err = tr.db.Exec("insert into device(devid, status, hostid, read_port) values(2, 'alive', 1, 5678)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into tempfile(fid, devid) values(9, 2)")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/create-close?fid=9&devid=2&key=foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	var resp CreateClose
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	expected := "http://foo:5678/dev2/0/000/000/0000000009.fid"
	if resp.Path != expected {
		t.Errorf("handler returned unexpected path: got %v want %v",
			resp.Path, expected)
	}
}

func TestCreateCloseOverwrite(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	insertHost(t, tr)
	_, err = tr.db.Exec("insert into device(devid, status, hostid) values(2, 'alive', 1)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file(fid, dkey) values(8, 'foo')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file_on(fid, devid) values(8, 2)")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tr.db.Exec("insert into device(devid, status, hostid, read_port) values(3, 'alive', 1, 5678)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into tempfile(fid, devid) values(9, 3)")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/create-close?fid=9&key=foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	var resp CreateClose
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	expected := "http://foo:5678/dev3/0/000/000/0000000009.fid"
	if resp.Path != expected {
		t.Errorf("handler returned unexpected path: got %v want %v",
			resp.Path, expected)
	}
}

func TestDelete(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	insertHost(t, tr)
	_, err = tr.db.Exec("insert into device(devid, hostid) values(2, 1)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file(fid, dkey) values(42, 'foo')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file_on(fid, devid) values(42, 2)")
	if err != nil {
		t.Fatal(err)
	}

	go tr.Run()
	defer tr.Shutdown()
	<-tr.Ready

	req, err := http.NewRequest("POST", "/delete?key=foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := ""
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestGetDevices(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	insertHost(t, tr)
	_, err = tr.db.Exec("insert into device(devid, status, hostid, bytes_total, bytes_used, bytes_free, updated_at) values(2, 'alive', 1, 1000, 500, 500, from_unixtime(1510216046))")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("GET", "/get-devices", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := `{"devices":[{"devid":2,"hostid":1,"host_name":"foo","host_status":"alive","rackid":1,"rack_name":"rack1","zoneid":1,"zone_name":"zone1","status":"alive","bytes_total":1000,"bytes_used":500,"bytes_free":500,"updated_at":1510216046,"io_utilization":null}]}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestGetHosts(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	insertHost(t, tr)
	req, err := http.NewRequest("GET", "/get-hosts", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := "{\"hosts\":[{\"hostid\":1,\"status\":\"alive\",\"hostname\":\"foo\",\"hostip\":\"1.2.3.4\"}]}\n"

	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
