package main

import (
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"
)

func openDatabase(config DatabaseConfig) (*sql.DB, error) {
	// Parse existing DSN to add parameters
	cfg, err := mysql.ParseDSN(config.DSN)
	if err != nil {
		return nil, err
	}

	// Add the parsing parameters
	cfg.ParseTime = true

	// Format back to DSN string
	dsn := cfg.FormatDSN()

	// Open connection with modified DSN
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime))
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetMaxOpenConns(config.MaxOpenConns)
	return db, nil
}
