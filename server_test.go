package main

import (
	"testing"
	"time"
)

func setupServer(t *testing.T, fileID int64) *Server {
	s, err := NewServer(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, s.db)
	_, err = s.db.Exec("insert into file(fid, dmid, classid, devcount) values(?, 1, 1, 1)", fileID)
	if err != nil {
		t.Fatal(err)
	}
	// Insert into tempfile table
	_, err = s.db.Exec("insert into tempfile(fid, createtime, classid, dmid) values(1, 1501245392, 1, 1)")
	if err != nil {
		t.Fatal(err)
	}
	return s

}

func TestFidExistsOnDatabase(t *testing.T) {
	s := setupServer(t, 1)

	res, err := s.fidExistsOnDatabase(1)
	if err != nil {
		t.Fatal(err)
	}

	if !res {
		t.Error("File exists on database but return not-exists!")
	}
}

func TestShouldDeleteFileExistsOnDbNewOnDisk(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s := setupServer(t, 1)

	modTime := time.Now().Add(-time.Duration(200 * time.Second))
	res, err := s.shouldDeleteFile(1, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if res {
		t.Error("File exists on database but return not-exists!")
	}

}
func TestShouldDeleteFileExistsOnDbOldOnDisk(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s := setupServer(t, 1)

	modTime := time.Now().Add(-time.Duration(400 * time.Second))
	res, err := s.shouldDeleteFile(1, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if res {
		t.Error("File exists on database but return not-exists!")
	}
}
func TestShouldDeleteFileNotExistsOnDbNewOnDisk(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s := setupServer(t, 1)

	modTime := time.Now().Add(-time.Duration(200 * time.Second))
	res, err := s.shouldDeleteFile(2, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if res {
		t.Error("File exists on database but return not-exists!")
	}
}
func TestShouldDeleteFileNotExistsOnDbOldOnDisk(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s := setupServer(t, 1)

	modTime := time.Now().Add(-time.Duration(400 * time.Second))
	res, err := s.shouldDeleteFile(2, modTime)
	if err != nil {
		t.Fatal(err)
	}

	if !res {
		t.Error("File should be deleted but returned not to!")
	}

}
