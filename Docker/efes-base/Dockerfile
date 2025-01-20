FROM golang:1.23.1 AS builder

WORKDIR /efes

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go install .

################################

FROM ubuntu:22.04

COPY --from=builder /go/bin/efes /usr/local/bin/efes

COPY ./Docker/efes-server/config.toml /etc/efes.toml

ENTRYPOINT ["efes"]
