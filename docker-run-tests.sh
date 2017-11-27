#!/bin/bash -ex

mysql=( mysql -h efestest_mysql_1 -u efes -p123 efes )
until "${mysql[@]}" -e "select 1" &>/dev/null ; do
  echo "MySQL is not ready yet..."
  sleep 1
done
echo "MySQL is ready."

exec go test -v ./...
