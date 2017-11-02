#!/bin/bash -ex
devid=$1
datadir=/srv/mogilefs/dev$devid
mkdir -p $datadir
ip=$(hostname -i)
sqlhost="INSERT INTO host (hostid, status, hostname, hostip) VALUES ($devid, 'alive', '$HOSTNAME', '$ip') ON DUPLICATE KEY UPDATE status='alive', hostname='$HOSTNAME', hostip='$ip'"
sqldevice="INSERT INTO device (devid, hostid, status) VALUES ($devid, $devid, 'alive') ON DUPLICATE KEY UPDATE hostid=$devid, status='alive'"
bash /root/wait-for-db.sh efes_db_1 mogilefs 123 mogilefs "$sqlhost"
bash /root/wait-for-db.sh efes_db_1 mogilefs 123 mogilefs "$sqldevice"
efes server $datadir
