FROM ubuntu:xenial

ARG DEBIAN_FRONTEND=noninteractive
ARG MYSQL_APT_CONFIG_DEB=mysql-apt-config_0.8.12-1_all.deb

RUN apt-get update && apt-get install -y wget lsb-release && rm -rf /var/lib/apt/lists/*
RUN wget "https://dev.mysql.com/get/$MYSQL_APT_CONFIG_DEB"
RUN dpkg -i $MYSQL_APT_CONFIG_DEB && rm mysql-apt-config_0.8.12-1_all.deb
RUN apt-get update && apt-get install -y mysql-server mysql-client && rm -rf /var/lib/apt/lists/*

ARG MYSQL_USER=efes
ARG MYSQL_PASSWORD=123
ARG MYSQL_DATABASE=efes
ARG MYSQL_SQL_FILE=/tmp/schema.sql

# Load schema.
ADD init.sh /tmp/
ADD schema.sql /tmp/
RUN ["bash", "/tmp/init.sh"]

# Print logs to stderr.
RUN ln -sf /dev/stderr /var/log/mysql/error.log

# Wrapper to catch SIGINT and forward as SIGTERM.
COPY entrypoint.sh entrypoint.sh
ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]

CMD ["mysqld", "--user=root"]

VOLUME /var/lib/mysql
EXPOSE 3306
