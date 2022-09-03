FROM golang:1.15 AS builder
RUN apt-get update && apt-get install -y default-mysql-client
WORKDIR /go/src/github.com/putdotio/efes/
ENV GO111MODULE=on
COPY go.mod .
COPY go.sum .
RUN go mod download
ADD ./Docker/test/docker-run-tests.sh /root/run-tests.sh
COPY . .
ENTRYPOINT ["/bin/bash", "/root/run-tests.sh"]
