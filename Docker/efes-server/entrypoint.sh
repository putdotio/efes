#!/bin/bash -ex
zoneid=4
rackid=3
hostid=2
devid=1

datadir=/srv/efes/dev$devid
mkdir -p $datadir
ip=$(hostname -i)

sqlzone="INSERT INTO zone (zoneid, name)
         VALUES ($zoneid, 'zone$zoneid')
         ON DUPLICATE KEY UPDATE name = VALUES(name)"

sqlrack="INSERT INTO rack (rackid, zoneid, name)
         VALUES ($rackid, $zoneid, 'rack$rackid')
         ON DUPLICATE KEY UPDATE zoneid = VALUES(zoneid), name = VALUES(name)"

sqlhost="INSERT INTO host (hostid, rackid, status, hostname, hostip)
         VALUES ($hostid, $rackid, 'alive', '$HOSTNAME', '$ip')
         ON DUPLICATE KEY UPDATE rackid = VALUES(rackid), status = VALUES(status),
                                hostname = VALUES(hostname), hostip = VALUES(hostip)"

sqldevice="INSERT INTO device (devid, hostid, status)
           VALUES ($devid, $hostid, 'alive')
           ON DUPLICATE KEY UPDATE hostid = VALUES(hostid), status = VALUES(status)"

efes ready mysql --exec "$sqlzone"
efes ready mysql --exec "$sqlrack"
efes ready mysql --exec "$sqlhost"
efes ready mysql --exec "$sqldevice"
efes ready rabbitmq

exec efes server $datadir
