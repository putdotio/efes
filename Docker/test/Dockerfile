FROM golang:1.19 AS builder
RUN apt-get update && apt-get install -y default-mysql-client
WORKDIR /go/src/github.com/putdotio/efes/
COPY go.mod .
COPY go.sum .
RUN go mod download
ADD ./Docker/test/docker-run-tests.sh /root/run-tests.sh
COPY . .
ADD ./Docker/test/config.toml /etc/efes.toml
ENTRYPOINT ["/bin/bash", "/root/run-tests.sh"]
