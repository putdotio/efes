#!/bin/bash -ex

mysql=( mysql -h mysql -u efes -p123 efes )
until "${mysql[@]}" -e "select 1" &>/dev/null ; do
  echo "MySQL is not ready yet..."
  sleep 1
done
echo "MySQL is ready."

exec go test -v -race -covermode atomic -coverprofile=/coverage/covprofile ./...
