package main

import (
	"fmt"
	"os"
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
	_, err = s.db.Exec("insert into tempfile(fid, createtime, classid, dmid) values(?, 1501245392, 1, 1)", fileID)
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
	res := s.shouldDeleteFile(1, modTime)

	if res {
		t.Error("File exists on database && new on disk but returned true to delete!")
	}

}
func TestShouldDeleteFileExistsOnDbOldOnDisk(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s := setupServer(t, 1)

	modTime := time.Now().Add(-time.Duration(400 * time.Second))
	res := s.shouldDeleteFile(1, modTime)

	if res {
		t.Error("File exists on database && old on disk but returned true to delete!")
	}
}
func TestShouldDeleteFileNotExistsOnDbNewOnDisk(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s := setupServer(t, 1)

	modTime := time.Now().Add(-time.Duration(200 * time.Second))
	res := s.shouldDeleteFile(2, modTime)

	if res {
		t.Error("File exists on database && new on disk but returned true to delete!")
	}
}
func TestShouldDeleteFileNotExistsOnDbOldOnDisk(t *testing.T) {
	testConfig.Server.CleanDiskFileTTL = 300
	s := setupServer(t, 1)

	modTime := time.Now().Add(-time.Duration(400 * time.Second))
	res := s.shouldDeleteFile(2, modTime)

	if !res {
		t.Error("File should be deleted but returned not to!")
	}

}

func TestDeleteFidOnDisk(t *testing.T) {
	s := setupServer(t, 1)
	var fid int64
	fid = 123
	sfid := fmt.Sprintf("%010d", fid)
	path := fmt.Sprintf("%s/%s/%s/%s/%s.fid", s.config.Server.DataDir, sfid[0:1], sfid[1:4], sfid[4:7], sfid)
	_, err := os.Create(path)

	if err != nil {
		t.Error("Error while creating file", err)
	}

	err = s.deleteFidOnDisk(fid)
	if err != nil {
		t.Error("File should be deleted but returned not to!")
	}
	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		t.Error("File should be deleted but exists on disk!")
	}

}
