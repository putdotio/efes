#!/bin/bash -ex

mysql=( mysql -h efestest_mysql_1 -u mogilefs -p123 mogilefs )
until "${mysql[@]}" -e "select 1" &>/dev/null ; do
  echo "MySQL is not ready yet, waiting..."
  sleep 1
done
echo "MySQL is ready."

# TODO wait for RabbitMQ to be ready

exec go test -v ./...
