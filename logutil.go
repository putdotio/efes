package main

import (
	"database/sql"
	"os"

	"github.com/cenkalti/log"
)

func logCloseFile(log log.Logger, f *os.File) {
	err := f.Close()
	if err != nil {
		log.Errorf("Error while closing file (%s): %s", f.Name(), err.Error())
	}
}

func logRollbackTx(log log.Logger, tx *sql.Tx) {
	err := tx.Rollback()
	if err != nil {
		log.Errorf("Error while closing transaction: %s", err.Error())
	}
}

func logCloseDB(log log.Logger, db *sql.DB) {
	err := db.Close()
	if err != nil {
		log.Errorf("Error while closing DB connection: %s", err.Error())
	}
}
