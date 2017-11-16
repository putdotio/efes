package main

import (
	"database/sql"
	"testing"
	"time"
)

// TODO: Duplicate with tracker test
func cleanDatabase(t *testing.T, db *sql.DB) {
	tables := []string{"file", "file_on", "tempfile", "device", "host"}
	for _, table := range tables {
		_, err := db.Exec("truncate table " + table)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestFidExistsOnDatabase(t *testing.T) {
	s, err := NewServer(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDatabase(t, s.db)
	// Insert into file table
	_, err = s.db.Exec("insert into file(fid, dmid, classid, devcount) values(1, 1, 1, 1)")
	if err != nil {
		t.Fatal(err)
	}
	// Insert into tempfile table
	_, err = s.db.Exec("insert into tempfile(fid, createtime, classid, dmid) values(1, 1501245392, 1, 1)")
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.fidExistsOnDatabase(1)
	if err != nil {
		t.Fatal(err)
	}

	if !res {
		t.Error("File exists on database but return not-exists!")
	}
}

func TestShouldDeleteFile(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s, err := NewServer(testConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Add to database first
	cleanDatabase(t, s.db)
	// Insert into file table
	_, err = s.db.Exec("insert into file(fid, dmid, classid, devcount) values(1, 1, 1, 1)")
	if err != nil {
		t.Fatal(err)
	}
	// Insert into tempfile table
	_, err = s.db.Exec("insert into tempfile(fid, createtime, classid, dmid) values(1, 1501245392, 1, 1)")
	if err != nil {
		t.Fatal(err)
	}

	// Case-1: File exist on db && new on disk
	modTime := time.Now().Add(-time.Duration(200 * time.Second))
	res, err := s.shouldDeleteFile(1, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if res {
		t.Error("File exists on database but return not-exists!")
	}

	// Case-2: File exist on db && old on disk
	modTime = time.Now().Add(-time.Duration(400 * time.Second))
	res, err = s.shouldDeleteFile(1, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if res {
		t.Error("File exists on database but return not-exists!")
	}

	// Case-3: File not exist on db && new on disk
	modTime = time.Now().Add(-time.Duration(200 * time.Second))
	res, err = s.shouldDeleteFile(2, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if res {
		t.Error("File exists on database but return not-exists!")
	}

	// Case-4: File not exist on db && old on disk
	modTime = time.Now().Add(-time.Duration(400 * time.Second))
	res, err = s.shouldDeleteFile(2, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if !res {
		t.Error("File should be deleted but returned not to!")
	}
}
