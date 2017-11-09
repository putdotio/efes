#!/bin/bash
set -e

# This file will be executed after intializing the database.
MYSQL_SQL_FILE=/tmp/schema.sql

# Use a datadir other than /var/lib/mysql to persist data in image.
# It is not possible with the original datadir location because
# it is defined as VOLUME in base "mysql" image.
MYSQL_DATADIR=/var/lib/mysql2

mkdir -p "$MYSQL_DATADIR"

echo 'Initializing database'
mysqld --initialize-insecure --datadir=$MYSQL_DATADIR &>/dev/null
echo 'Database initialized'

SOCKET=/var/run/mysqld/mysqld.sock
mysqld --user=root --datadir=$MYSQL_DATADIR --skip-networking --socket="${SOCKET}" &>/dev/null &
pid="$!"

mysql=( mysql --protocol=socket -uroot -hlocalhost --socket="${SOCKET}" )

for i in {30..0}; do
        if echo 'SELECT 1' | "${mysql[@]}" &> /dev/null; then
                break
        fi
        echo 'MySQL init process in progress...'
        sleep 1
done
if [ "$i" = 0 ]; then
        echo >&2 'MySQL init process failed.'
        exit 1
fi

"${mysql[@]}" <<-EOSQL
        SET @@SESSION.SQL_LOG_BIN=0;
        DELETE FROM mysql.user WHERE user NOT IN ('mysql.sys', 'mysqlxsys', 'root') OR host NOT IN ('localhost') ;
        GRANT ALL ON *.* TO 'root'@'localhost' WITH GRANT OPTION ;
        DROP DATABASE IF EXISTS test ;
        FLUSH PRIVILEGES ;
EOSQL

echo "CREATE DATABASE IF NOT EXISTS \`$MYSQL_DATABASE\` ;" | "${mysql[@]}"
mysql+=( "$MYSQL_DATABASE" )

echo "CREATE USER '$MYSQL_USER'@'%' IDENTIFIED BY '$MYSQL_PASSWORD' ;" | "${mysql[@]}"
echo "GRANT ALL ON \`$MYSQL_DATABASE\`.* TO '$MYSQL_USER'@'%' ;" | "${mysql[@]}"
echo 'FLUSH PRIVILEGES ;' | "${mysql[@]}"

echo "$0: running $MYSQL_SQL_FILE"; "${mysql[@]}" < "$MYSQL_SQL_FILE"; echo

if ! kill -s TERM "$pid" || ! wait "$pid"; then
        echo >&2 'MySQL init process failed.'
        exit 1
fi

echo
echo 'MySQL init process done. Ready for start up.'
echo
