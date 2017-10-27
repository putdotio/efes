FROM mysql:5.7.15

ENV MYSQL_DATABASE=mogilefs \
    MYSQL_ROOT_PASSWORD=123

ADD schema.sql /docker-entrypoint-initdb.d

EXPOSE 3306
