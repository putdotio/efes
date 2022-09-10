#!/bin/bash -ex
zoneid=4
rackid=3
hostid=2
devid=1

datadir=/srv/efes/dev$devid
mkdir -p $datadir
ip=$(hostname -i)

sqlzone="REPLACE INTO zone (zoneid, name) VALUES ($zoneid, 'zone$zoneid')"
sqlrack="REPLACE INTO rack (rackid, zoneid, name) VALUES ($rackid, $zoneid, 'rack$rackid')"
sqlhost="REPLACE INTO host (hostid, rackid, status, hostname, hostip) VALUES ($hostid, $rackid, 'alive', '$HOSTNAME', '$ip')"
sqldevice="REPLACE INTO device (devid, hostid, status) VALUES ($devid, $hostid, 'alive')"

efes ready mysql --exec "$sqlzone" 2>/dev/null
efes ready mysql --exec "$sqlrack" 2>/dev/null
efes ready mysql --exec "$sqlhost" 2>/dev/null
efes ready mysql --exec "$sqldevice" 2>/dev/null
efes ready rabbitmq 2>/dev/null

exec efes server $datadir
