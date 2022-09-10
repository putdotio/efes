FROM mysql:8
ADD schema.sql /docker-entrypoint-initdb.d
ADD entrypoint-trap.sh /
ENTRYPOINT ["/bin/bash", "/entrypoint-trap.sh"]
CMD ["mysqld", "--user=root"]