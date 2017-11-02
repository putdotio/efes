#!/bin/bash
set -e

host="$1"
user="$2"
password="$3"
db="$4"
shift 4
query="$@"

until mysql -h $host -u $user -p$password -e 'select version()' &>/dev/null ; do
  # echo "MySQL is unavailable - sleeping"
  sleep 1
done

for q in "$@"; do
  echo "Executing:" $q
  mysql -h $host -u $user -p$password $db -e "$q" 2>/dev/null
done
