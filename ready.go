package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/cenkalti/redialer/amqpredialer"
)

func readyMysql(cfg DatabaseConfig, timeout time.Duration, exec string) error {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
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
	amqp, err := amqpredialer.New(cfg.URL)
	if err != nil {
		return err
	}
	go amqp.Run()
	defer amqp.Close()
	timeoutC := time.After(timeout)
	for {
		select {
		case <-amqp.Conn():
			return nil
		case <-timeoutC:
			return fmt.Errorf("mysql did not become ready in %s", timeout)
		}
	}
}
