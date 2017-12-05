package main

import (
	"net/http"
	"net/http/httptest"
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

func TestGetPaths(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip) values(1, 'foo', 'alive', '1.2.3.4')")
	if err != nil {
		t.Fatal(err)
	}
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
	req, err := http.NewRequest("GET", "/get-paths?key=foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := "{\"paths\":[\"http://1.2.3.4:1234/dev2/0/000/000/0000000042.fid\"]}\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestCreateOpen(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip) values(1, 'foo', 'alive', '1.2.3.4')")
	if err != nil {
		t.Fatal(err)
	}
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
	expected := "{\"path\":\"http://1.2.3.4:1234/dev2/0/000/000/0000000005.fid\",\"fid\":5,\"devid\":2}\n"
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
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip) values(1, 'foo', 'alive', '1.2.3.4')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid) values(2, 'alive', 1)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into tempfile(fid, devid) values(9, 2)")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/create-close?fid=9&devid=2&key=foo&size=42", nil)
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

func TestDelete(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip) values(1, 'foo', 'alive', '1.2.3.4')")
	if err != nil {
		t.Fatal(err)
	}
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
	_, err = tr.db.Exec("insert into host(hostid, hostname, hostip, status) values(1, 'foo', '127.0.0.1', 'alive')")
	if err != nil {
		t.Fatal(err)
	}
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
	expected := "{\"devices\":[{\"devid\":2,\"hostid\":1,\"status\":\"alive\",\"bytes_total\":1000,\"bytes_used\":500,\"bytes_free\":500,\"updated_at\":1510216046,\"io_utilization\":null}]}\n"
	if rr.Body.String() != expected {
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
	_, err = tr.db.Exec("insert into host(hostid, hostname, hostip, status) values(1, 'foo', '127.0.0.1', 'alive')")
	if err != nil {
		t.Fatal(err)
	}
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
	expected := "{\"hosts\":[{\"hostid\":1,\"status\":\"alive\",\"hostname\":\"foo\",\"hostip\":\"127.0.0.1\"}]}\n"

	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
