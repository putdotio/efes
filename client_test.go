package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func createTempfile(t *testing.T, content string) string {
	t.Helper()
	f, err := ioutil.TempFile("", "efestest")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(content)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestClient(t *testing.T) {
	const chunkSize = 2
	content := "12345" // a string length of 5 for testing chunk size of 2
	source := createTempfile(t, content)
	defer os.Remove(source)

	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	go tr.Run()
	defer tr.Shutdown()
	_, err = tr.db.Exec("insert into host(hostid, status, hostip) values(1, 'alive', '127.0.0.1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, read_port, write_port) values(2, 'alive', 1, 1000, 500, 8500, 8501)")
	if err != nil {
		t.Fatal(err)
	}
	devPath := "/srv/efes/dev2"
	err = os.MkdirAll(devPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
	testConfig.Server.DataDir = devPath
	srv, err := NewServer(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Run()
	defer srv.Shutdown()
	testConfig.Client.ChunkSize = chunkSize
	clt, err := NewClient(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	<-tr.Ready
	<-srv.Ready

	exist, err := clt.Exist("foo")
	if err != nil {
		t.Fatal(err)
	}
	if exist {
		t.Fatal("key must not exist")
	}

	err = clt.Write("foo", source)
	if err != nil {
		t.Fatal(err)
	}

	exist, err = clt.Exist("foo")
	if err != nil {
		t.Fatal(err)
	}
	if !exist {
		t.Fatal("key must exist")
	}

	copied := createTempfile(t, "")
	defer os.Remove(copied)
	err = clt.Read("foo", copied)
	if err != nil {
		t.Fatal(err)
	}

	copyContent, err := ioutil.ReadFile(copied)
	if err != nil {
		t.Fatal(err)
	}
	if string(copyContent) != content {
		t.Fatal("invalid content:", copyContent)
	}

	err = clt.Delete("foo")
	if err != nil {
		t.Fatal(err)
	}

	exist, err = clt.Exist("foo")
	if err != nil {
		t.Fatal(err)
	}
	if exist {
		t.Fatal("key must not exist")
	}
}
