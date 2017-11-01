#!/bin/bash
set -e

host="$1"
user="$2"
password="$3"
db="$4"
shift
shift
shift
shift
query="$@"

until mysql -h $host -u $user -p$password -e 'select version();'; do
  >&2 echo "MySQL is unavailable - sleeping"
  sleep 1
done

for q in "$@"; do
  echo "Executing: " $q
  mysql -h $host -u $user -p$password $db -e "$q"
done
