package tracker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/putdotio/efes/config"
)

func TestPing(t *testing.T) {
	cfg := &config.Config{}
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

func TestCreateOpen(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			DSN: "mogilefs:123@(efestest_db_1:3306)/mogilefs",
		},
	}
	tr, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
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
	req, err := http.NewRequest("GET", "/create-open", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	tr.server.Handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	expected := "{\"path\":\"http://1.2.3.4:7500/dev2/0/000/000/0000000005.fid\"}\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
