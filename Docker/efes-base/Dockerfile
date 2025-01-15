FROM golang:1.23.1 AS builder
WORKDIR /go/src/github.com/putdotio/efes/
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go install .

FROM ubuntu:xenial
COPY --from=builder /go/bin/efes /usr/local/bin/efes
ENTRYPOINT ["efes"]
