FROM mysql:8

ENV MYSQL_USER=efes \
    MYSQL_PASSWORD=123 \
    MYSQL_DATABASE=efes \
    MYSQL_RANDOM_ROOT_PASSWORD=1

# Copy schema file into the initialization directory
COPY ./Docker/mysql/init/schema.sql /docker-entrypoint-initdb.d/
