#!/bin/bash -ex
devid=$1
datadir=/srv/efes/dev$devid
mkdir -p $datadir
ip=$(hostname -i)

sqlhost="REPLACE INTO host (hostid, status, hostname, hostip) VALUES ($devid, 'alive', '$HOSTNAME', '$ip')"
sqldevice="REPLACE INTO device (devid, hostid, status) VALUES ($devid, $devid, 'alive')"

efes ready mysql --exec "$sqlhost" 2>/dev/null
efes ready mysql --exec "$sqldevice" 2>/dev/null
efes ready rabbitmq 2>/dev/null

exec efes server $datadir
