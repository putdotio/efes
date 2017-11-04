package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPing(t *testing.T) {
	cfg := &Config{}
	tr, err := New(cfg)
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
	cfg := &Config{
		Database: DatabaseConfig{
			DSN: "mogilefs:123@(efestest_db_1:3306)/mogilefs",
		},
	}
	tr, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, mb_asof) values(2, 'alive', 1, 1000, 500, ?)", time.Now().UTC().Unix())
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
	cfg := &Config{
		Database: DatabaseConfig{
			DSN: "mogilefs:123@(efestest_db_1:3306)/mogilefs",
		},
	}
	tr, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, mb_asof) values(2, 'alive', 1, 1000, 500, ?)", time.Now().UTC().Unix())
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
	expected := "{\"path\":\"http://1.2.3.4:7500/dev2/0/000/000/0000000005.fid\",\"fid\":5,\"devid\":2}\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestCreateClose(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			DSN: "mogilefs:123@(efestest_db_1:3306)/mogilefs",
		},
	}
	tr, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, mb_asof) values(2, 'alive', 1, 1000, 500, ?)", time.Now().UTC().Unix())
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
	cfg := &Config{
		Database: DatabaseConfig{
			DSN: "mogilefs:123@(efestest_db_1:3306)/mogilefs",
		},
	}
	tr, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	_, err = tr.db.Exec("insert into host(hostid, status, hostip, http_port) values(1, 'alive', '1.2.3.4', 7500)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, mb_asof) values(2, 'alive', 1, 1000, 500, ?)", time.Now().UTC().Unix())
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
