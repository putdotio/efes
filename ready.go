package main

import (
	"fmt"
	"time"

	"github.com/cenkalti/log"
	"github.com/streadway/amqp"
)

func readyMysql(cfg DatabaseConfig, timeout time.Duration, exec string) error {
	db, err := openDatabase(cfg)
	if err != nil {
		return err
	}
	defer logCloseDB(log.DefaultLogger, db)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeoutC := time.After(timeout)
	for {
		select {
		case <-ticker.C:
			err = db.Ping()
			if err != nil {
				continue
			}
			if exec != "" {
				_, err = db.Exec(exec)
				return err
			}
			return nil
		case <-timeoutC:
			return fmt.Errorf("mysql did not become ready in %s", timeout)
		}
	}
}

func readyRabbitmq(cfg AMQPConfig, timeout time.Duration) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeoutC := time.After(timeout)
	for {
		select {
		case <-ticker.C:
			conn, err := amqp.Dial(cfg.URL)
			if err != nil {
				continue
			}
			return conn.Close()
		case <-timeoutC:
			return fmt.Errorf("mysql did not become ready in %s", timeout)
		}
	}
}
