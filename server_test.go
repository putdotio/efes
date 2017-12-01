package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func setupServer(t *testing.T, ttl time.Duration) *Server {
	t.Helper()

	c2 := *testConfig
	c2.Server.CleanDiskFileTTL = Duration(ttl)

	s, err := NewServer(&c2)
	if err != nil {
		t.Fatal(err)
	}

	cleanDB(t, s.db)
	return s
}

func TestFidExistsOnDatabase(t *testing.T) {
	s := setupServer(t, 0)
	insertToDB(t, s.db, 1)

	res, err := s.fidExistsOnDatabase(1)
	if err != nil {
		t.Fatal(err)
	}

	if !res {
		t.Error("File exists on database but return not-exists!")
	}
}

func TestShouldDeleteFileExistsOnDbNewOnDisk(t *testing.T) {
	s := setupServer(t, 300*time.Second)
	insertToDB(t, s.db, 1)
	fidPath := writeToDisk(t, 1, "fid", time.Now().Add(-200*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func insertToDB(t *testing.T, db *sql.DB, fid int64) {
	t.Helper()
	_, err := db.Exec("insert into file(fid, dmid, classid, devcount) values(?, 1, 1, 1)", fid)
	if err != nil {
		t.Fatal(err)
	}
}

func writeToDisk(t *testing.T, fid int64, ext string, modTime time.Time) string {
	t.Helper()
	dirPath := "/srv/efes/dev1/0/000/000"
	err := os.MkdirAll(dirPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
	fidPath := "/srv/efes/dev1/0/000/000/000000000" + strconv.FormatInt(fid, 10) + "." + ext
	f, err := os.Create(fidPath)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chtimes(fidPath, modTime, modTime)
	if err != nil {
		t.Fatal(err)
	}
	return fidPath
}

func TestShouldDeleteFileExistsOnDbOldOnDisk(t *testing.T) {
	s := setupServer(t, 300*time.Second)
	insertToDB(t, s.db, 1)
	fidPath := writeToDisk(t, 1, "fid", time.Now().Add(-400*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteFileNotExistsOnDbNewOnDisk(t *testing.T) {
	s := setupServer(t, 300*time.Second)
	fidPath := writeToDisk(t, 1, "fid", time.Now().Add(-200*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteFileNotExistsOnDbOldOnDisk(t *testing.T) {
	s := setupServer(t, 300*time.Second)
	fidPath := writeToDisk(t, 1, "fid", time.Now().Add(-400*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteDir(t *testing.T) {
	s := setupServer(t, 300*time.Second)
	dirPath := "/srv/efes/dev1/0/000/000"
	err := os.MkdirAll(dirPath, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(dirPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteJunkOld(t *testing.T) {
	s := setupServer(t, 300*time.Second)
	fidPath := writeToDisk(t, 1, "notfid", time.Now().Add(-400*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteJunkNew(t *testing.T) {
	s := setupServer(t, 300*time.Second)
	fidPath := writeToDisk(t, 1, "notfid", time.Now().Add(-200*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestDeleteFidOnDisk(t *testing.T) {
	s := setupServer(t, 1)
	var fid int64 = 123
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
