FROM golang:1.23.1 AS builder

# Install mysql
RUN apt-get update && \
    apt-get -y install --no-install-recommends default-mysql-client && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /efes

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY ./Docker/test/entrypoint.sh /root/entrypoint.sh
COPY . .
COPY ./Docker/test/config.toml /etc/efes.toml

ENTRYPOINT ["/bin/bash", "/root/entrypoint.sh"]
