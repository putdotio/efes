FROM golang:1.9.2 AS builder
RUN go get github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/putdotio/efes/
ADD Gopkg.toml Gopkg.lock /go/src/github.com/putdotio/efes/
RUN dep ensure -vendor-only
ADD main.go /go/src/github.com/putdotio/efes/
ADD config /go/src/github.com/putdotio/efes/config/
ADD tracker /go/src/github.com/putdotio/efes/tracker/
ADD server /go/src/github.com/putdotio/efes/server/
RUN CGO_ENABLED=0 go install .

FROM ubuntu:xenial
COPY --from=builder /go/bin/efes /usr/local/bin/efes
ADD config.toml /etc/efes.toml
ENTRYPOINT ["/usr/local/bin/efes"]
