#!/bin/bash -ex
bash /root/wait-for-mysql.sh efestest_mysql_1 mogilefs 123 mogilefs
exec go test -v ./...
