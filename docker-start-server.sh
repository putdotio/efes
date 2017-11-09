#!/bin/bash -ex
devid=$1
datadir=/srv/mogilefs/dev$devid
mkdir -p $datadir
ip=$(hostname -i)

sqlhost="INSERT INTO host (hostid, status, hostname, hostip) VALUES ($devid, 'alive', '$HOSTNAME', '$ip') ON DUPLICATE KEY UPDATE status='alive', hostname='$HOSTNAME', hostip='$ip'"
sqldevice="INSERT INTO device (devid, hostid, status) VALUES ($devid, $devid, 'alive') ON DUPLICATE KEY UPDATE hostid=$devid, status='alive'"

mysql=( mysql -h efes_mysql_1 -u mogilefs -p123 mogilefs )

until "${mysql[@]}" -e "select 1" &>/dev/null ; do
  echo "MySQL is not ready yet, waiting..."
  sleep 1
done
echo "MySQL is ready."

"${mysql[@]}" -e "$sqlhost" 2>/dev/null
"${mysql[@]}" -e "$sqldevice" 2>/dev/null

exec efes server $datadir
