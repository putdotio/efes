package main

import (
	"database/sql"
	"testing"
)

var testConfig *Config

func init() {
	testConfig = NewConfig()
	err := testConfig.ReadFile("/etc/efes.toml")
	if err != nil {
		panic(err)
	}
}

func cleanDB(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{"file_on", "tempfile", "file", "device", "host", "rack", "zone"}
	for _, table := range tables {
		_, err := db.Exec("delete from " + table)
		if err != nil {
			t.Fatal(err)
		}
	}
}
