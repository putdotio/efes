package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var testConfig = &Config{
	Database: DatabaseConfig{
		DSN: "mogilefs:123@(efestest_mysql_1:3306)/mogilefs",
	},
	AMQP: AMQPConfig{
		URL: "amqp://guest:guest@efestest_rabbitmq_1:5672/",
	},
}

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

func cleanDB(t *testing.T, db *sql.DB) {
	tables := []string{"file", "file_on", "tempfile", "device", "host"}
	for _, table := range tables {
		_, err := db.Exec("truncate table " + table)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestGetPaths(t *testing.T) {
	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used) values(2, 'alive', 1, 1000, 500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file(fid, dmid, dkey, length, classid, devcount) values(42, ?, 'foo', 500, ?, 1)", dmid, classid)
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
	expected := "{\"paths\":[\"http://1.2.3.4:7500/dev2/0/000/000/0000000042.fid\"]}\n"
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
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, http_port) values(2, 'alive', 1, 1000, 500, 1234)")
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
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used) values(2, 'alive', 1, 1000, 500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into tempfile(fid, createtime, dmid, classid) values(9, ?, ?, ?)", time.Now().UTC().Unix(), dmid, classid)
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
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used) values(2, 'alive', 1, 1000, 500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file(fid, dmid, dkey, length, classid, devcount) values(42, ?, 'foo', 500, ?, 1)", dmid, classid)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into file_on(fid, devid) values(42, 2)")
	if err != nil {
		t.Fatal(err)
	}
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
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, updated_at) values(2, 'alive', 1, 1000, 500, from_unixtime(1510216046))")
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
	expected := "{\"devices\":[{\"devid\":2,\"hostid\":1,\"status\":\"alive\",\"mb_total\":1000,\"mb_used\":500,\"mb_asof\":1510216046,\"io_utilization\":null}]}\n"
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
	_, err = tr.db.Exec("insert into host(hostid, hostname, hostip, http_port, status) values(1, 'foo', '127.0.0.1', 6543, 'alive')")
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
	expected := "{\"hosts\":[{\"hostid\":1,\"status\":\"alive\",\"http_port\":6543,\"hostname\":\"foo\",\"hostip\":\"127.0.0.1\"}]}\n"

	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
