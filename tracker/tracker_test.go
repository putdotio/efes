package tracker

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
