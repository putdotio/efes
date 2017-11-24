package main

import (
	"database/sql"
	"testing"
)

var testConfig *Config

func init() {
	c := defaultConfig
	c.Debug = true
	c.Database.DSN = "mogilefs:123@(efestest_mysql_1:3306)/mogilefs"
	c.AMQP.URL = "amqp://efes:123@efestest_rabbitmq_1:5672/"
	testConfig = &c
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
