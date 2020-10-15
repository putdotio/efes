FROM golang:1.15 AS builder
WORKDIR /go/src/github.com/putdotio/efes/
ENV GO111MODULE=on
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go install .

FROM ubuntu:xenial
COPY --from=builder /go/bin/efes /usr/local/bin/efes
ENTRYPOINT ["efes"]
