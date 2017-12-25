package main

import (
	"database/sql"
	"time"
)

func openDatabase(config DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", config.DSN)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime))
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetMaxOpenConns(config.MaxOpenConns)
	return db, nil
}
