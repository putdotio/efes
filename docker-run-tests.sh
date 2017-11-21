#!/bin/bash -ex

mysql=( mysql -h efestest_mysql_1 -u mogilefs -p123 mogilefs )
until "${mysql[@]}" -e "select 1" &>/dev/null ; do
  echo "MySQL is not ready yet..."
  sleep 1
done
echo "MySQL is ready."

echo Starting RabbitMQ server...
rabbitmq-server --hostname localhost &>/dev/null &
until rabbitmqctl -t 1 list_queues &> /dev/null; do
  echo 'RabbitMQ is not ready yet...'
  sleep 1
done
echo "RabbitMQ is ready."

echo Adding user...
rabbitmqctl add_user efes 123
rabbitmqctl set_user_tags efes administrator
rabbitmqctl set_permissions -p / efes  ".*" ".*" ".*"

exec go test -v ./...
