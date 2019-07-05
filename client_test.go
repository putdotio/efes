package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
	content := "12345"   // a string length of 5 for testing chunk size of 2
	content2 := "qwerty" // a string length of 5 for testing chunk size of 2
	source := createTempfile(t, content)
	source2 := createTempfile(t, content2)
	defer os.Remove(source)
	defer os.Remove(source2)

	tr, err := NewTracker(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	cleanDB(t, tr.db)
	go tr.Run()
	defer tr.Shutdown()
	_, err = tr.db.Exec("insert into zone(zoneid, name) values(1, 'zone1')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into rack(rackid, zoneid, subnet) values(1, 1, '0.0.0.0/0')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into host(hostid, hostname, status, hostip, rackid) values(1, '127.0.0.1', 'alive', '127.0.0.1', 1)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tr.db.Exec("insert into device(devid, status, hostid, bytes_total, bytes_used, bytes_free, read_port, write_port) values(2, 'alive', 1, 1000000000, 500000000, 500000000, 8500, 8501)")
	if err != nil {
		t.Fatal(err)
	}
	tempDir, err := ioutil.TempDir("", "efes-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	devPath := filepath.Join(tempDir, "dev2")
	err = os.Mkdir(devPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := *testConfig
	serverConfig.Server.DataDir = devPath
	srv, err := NewServer(&serverConfig)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Run()
	defer srv.Shutdown()

	clientConfig := NewConfig()
	clientConfig.Client.ChunkSize = chunkSize
	clt, err := NewClient(clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	<-tr.Ready
	<-srv.Ready

	exist, err := clt.Exists("foo")
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

	exist, err = clt.Exists("foo")
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

	// test overwrite case
	err = clt.Write("foo", source2)
	if err != nil {
		t.Fatal(err)
	}

	copied2 := createTempfile(t, "")
	defer os.Remove(copied2)
	err = clt.Read("foo", copied2)
	if err != nil {
		t.Fatal(err)
	}

	copyContent2, err := ioutil.ReadFile(copied2)
	if err != nil {
		t.Fatal(err)
	}
	if string(copyContent2) != content2 {
		t.Fatal("invalid content:", copyContent2)
	}

	// test delete
	err = clt.Delete("foo")
	if err != nil {
		t.Fatal(err)
	}

	exist, err = clt.Exists("foo")
	if err != nil {
		t.Fatal(err)
	}
	if exist {
		t.Fatal("key must not exist")
	}

	// test injecting checksums in keys
	contentFox := "the quick brown fox jumps over the lazy dog\n"
	expectedSha1 := "5d2781d78fa5a97b7bafa849fe933dfc9dc93eba"
	err = clt.WriteReader("foo-{{.Sha1}}-bar", strings.NewReader(contentFox))
	if err != nil {
		t.Fatal(err)
	}
	exist, err = clt.Exists("foo-" + expectedSha1 + "-bar")
	if err != nil {
		t.Fatal(err)
	}
	if !exist {
		t.Fatal("key must exist")
	}
}
