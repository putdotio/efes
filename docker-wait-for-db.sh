#!/bin/bash
set -e

host="$1"
user="$2"
password="$3"
db="$4"
query="$5"

until mysql -h $host -u $user -p$password -c '\q'; do
  >&2 echo "MySQL is unavailable - sleeping"
  sleep 1
done

>&2 echo "MySQL is up - executing command"
mysql -h $host -u $user -p$password $db -e "$query"
