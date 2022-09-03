.PHONY: build up test lint

NAME := efes

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(NAME)

up:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	docker-compose -f ./Docker/docker-compose.yml rm -fsv
	docker-compose -f ./Docker/docker-compose.yml up --build

test:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	docker-compose -f ./Docker/docker-compose-test.yml rm -fsv
	docker-compose -f ./Docker/docker-compose-test.yml run --rm test

lint:
	golangci-lint run
