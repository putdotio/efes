#!/bin/bash -ex

mysql=( mysql -h efestest_mysql_1 -u mogilefs -p123 mogilefs )
until "${mysql[@]}" -e "select 1" &>/dev/null ; do
  echo "MySQL is not ready yet, waiting..."
  sleep 1
done
echo "MySQL is ready."

echo Creating data directory...
RABBITMQ_DATADIR=/var/lib/rabbitmq2
mkdir -p "$RABBITMQ_DATADIR"

echo Starting RabbitMQ server...
rabbitmq-server --hostname localhost &>/dev/null &
pid="$!"

echo Waiting for the server to get ready...
for i in {30..0}; do
        if rabbitmqctl -t 1 list_queues &> /dev/null; then
                break
        fi
        echo 'RabbitMQ is not ready yet...'
        sleep 1
done
if [ "$i" = 0 ]; then
        echo >&2 'RabbitMQ init process failed.'
        exit 1
fi

echo Adding user...
rabbitmqctl add_user efes 123
rabbitmqctl set_user_tags efes administrator
rabbitmqctl set_permissions -p / efes  ".*" ".*" ".*"

exec go test -v ./...
