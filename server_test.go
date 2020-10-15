package main

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupServer(t *testing.T, ttl time.Duration) (s *Server, closeFunc func()) {
	t.Helper()

	tempDir, err := ioutil.TempDir("", "efes-test-")
	if err != nil {
		t.Fatal(err)
	}
	devPath := filepath.Join(tempDir, "dev2")
	err = os.Mkdir(devPath, 0700)
	if err != nil {
		t.Fatal(err)
	}

	c2 := *testConfig
	c2.Server.DataDir = devPath
	c2.Server.CleanDiskFileTTL = Duration(ttl)

	s, err = NewServer(&c2)
	if err != nil {
		t.Fatal(err)
	}

	cleanDB(t, s.db)

	_, err = s.db.Exec("insert into zone(zoneid, name) values(1, 'zone1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("insert into rack(rackid, zoneid, name) values(1, 1, 'rack1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("insert into host(hostid, hostname, status, hostip, rackid) values(1, 'foo', 'alive', '127.0.0.1', 1)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("insert into device(devid, status, hostid, bytes_total, bytes_used, bytes_free, read_port, write_port) values(?, 'alive', 1, 1000000000, 500000000, 500000000, 8500, 8501)", s.devid)
	if err != nil {
		t.Fatal(err)
	}
	return s, func() { os.RemoveAll(tempDir) }
}

func TestFidExistsOnDatabase(t *testing.T) {
	s, rm := setupServer(t, 0)
	defer rm()
	insertToDB(t, s.db, 1, s.devid, "foo")

	res, err := s.fidExistsOnDatabase(1)
	if err != nil {
		t.Fatal(err)
	}

	if !res {
		t.Error("File exists on database but return not-exists!")
	}
}

func TestShouldDeleteFileExistsOnDbNewOnDisk(t *testing.T) {
	s, rm := setupServer(t, 300*time.Second)
	defer rm()
	insertToDB(t, s.db, 2, s.devid, "foo")
	fidPath := writeToDisk(t, s, 2, "fid", time.Now().Add(-200*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func insertToDB(t *testing.T, db *sql.DB, fid, devid int64, key string) {
	t.Helper()
	_, err := db.Exec("insert into file(fid, dkey) values(?, ?)", fid, key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("insert into file_on(fid, devid) values(?, ?)", fid, devid)
	if err != nil {
		t.Fatal(err)
	}
}

func writeToDisk(t *testing.T, s *Server, fid int64, ext string, modTime time.Time) string {
	t.Helper()
	fidPath := filepath.Join(s.config.Server.DataDir, vivifyExt(fid, ext))
	dirPath, _ := filepath.Split(fidPath)
	err := os.MkdirAll(dirPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
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
	s, rm := setupServer(t, 300*time.Second)
	defer rm()

	insertToDB(t, s.db, 1, s.devid, "foo")
	fidPath := writeToDisk(t, s, 1, "fid", time.Now().Add(-400*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteFileNotExistsOnDbNewOnDisk(t *testing.T) {
	s, rm := setupServer(t, 300*time.Second)
	defer rm()

	fidPath := writeToDisk(t, s, 1, "fid", time.Now().Add(-200*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteFileNotExistsOnDbOldOnDisk(t *testing.T) {
	s, rm := setupServer(t, 300*time.Second)
	defer rm()

	fidPath := writeToDisk(t, s, 1, "fid", time.Now().Add(-400*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteDir(t *testing.T) {
	s, rm := setupServer(t, 300*time.Second)
	defer rm()

	dirPath := filepath.Join(s.config.Server.DataDir, "1234")
	err := os.Mkdir(dirPath, 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(dirPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteJunkOld(t *testing.T) {
	s, rm := setupServer(t, 300*time.Second)
	defer rm()

	fidPath := writeToDisk(t, s, 1, "notfid", time.Now().Add(-400*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsExist(err) {
		t.Fatal(err)
	}
}

func TestShouldDeleteJunkNew(t *testing.T) {
	s, rm := setupServer(t, 300*time.Second)
	defer rm()

	fidPath := writeToDisk(t, s, 1, "notfid", time.Now().Add(-200*time.Second))

	err := filepath.Walk(s.config.Server.DataDir, s.visitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(fidPath)
	if os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestDeleteFidOnDisk(t *testing.T) {
	s, rm := setupServer(t, 1)
	defer rm()

	var fid int64 = 123
	path := filepath.Join(s.config.Server.DataDir, vivify(fid))
	dirPath, _ := filepath.Split(path)
	err := os.MkdirAll(dirPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Create(path)
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
