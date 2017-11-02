FROM mysql:5.7.20

ENV MYSQL_ROOT_PASSWORD=toor \
    MYSQL_USER=mogilefs \
    MYSQL_PASSWORD=123 \
    MYSQL_DATABASE=mogilefs

ADD schema.sql /docker-entrypoint-initdb.d

EXPOSE 3306
