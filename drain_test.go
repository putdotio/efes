package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestDrain(t *testing.T) {
	const chunkSize = 3
	content := "foo"
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

	// Put a file on first server
	testConfig.Client.ChunkSize = chunkSize
	clt, err := NewClient(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	<-tr.Ready
	<-srv.Ready

	fmt.Println("writing file to first server")
	err = clt.Write("k", source)
	if err != nil {
		t.Fatal(err)
	}

	// setup second server
	fmt.Println("setting up second server")
	_, err = tr.db.Exec("insert into device(devid, status, hostid, mb_total, mb_used, read_port, write_port) values(3, 'alive', 1, 1000, 500, 8502, 8503)")
	if err != nil {
		t.Fatal(err)
	}
	devPath = "/srv/efes/dev3"
	err = os.MkdirAll(devPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
	var config2 Config = *testConfig
	config2.Server.DataDir = devPath
	config2.Server.ListenAddressForRead = "0.0.0.0:8502"
	config2.Server.ListenAddress = "0.0.0.0:8503"
	srv2, err := NewServer(&config2)
	if err != nil {
		t.Fatal(err)
	}
	go srv2.Run()
	defer srv2.Shutdown()

	// Run drain
	dr, err := NewDrainer(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	err = dr.Run()
	if err != nil {
		t.Fatal(err)
	}

	// Check content
	copied := createTempfile(t, "")
	defer os.Remove(copied)
	err = clt.Read("k", copied)
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
}
