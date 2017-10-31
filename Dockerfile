FROM golang:1.9.2 AS builder
RUN go get github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/putdotio/efes/
ADD Gopkg.toml Gopkg.lock /go/src/github.com/putdotio/efes/
RUN dep ensure -vendor-only
ADD . /go/src/github.com/putdotio/efes/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o efes

FROM ubuntu:xenial
COPY --from=builder /go/src/github.com/putdotio/efes/efes /usr/local/bin/efes
ADD config.toml /etc/efes.toml
ENTRYPOINT ["/usr/local/bin/efes"]
