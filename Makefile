.PHONY: build up test lint

NAME := efes

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(NAME)

build-docker:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	$(eval CONTAINER_ID=$(shell docker create efes-base))
	mkdir ./bin
	docker cp $(CONTAINER_ID):/usr/local/bin/efes - > ./bin/efes
	docker rm -v $(CONTAINER_ID)

up:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	docker-compose -f ./Docker/docker-compose.yml rm -fsv
	docker-compose -f ./Docker/docker-compose.yml up --build

test:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	docker-compose -f ./Docker/docker-compose-test.yml rm -fsv
	docker-compose -f ./Docker/docker-compose-test.yml build
	docker-compose -f ./Docker/docker-compose-test.yml run --rm test

lint:
	golangci-lint run
