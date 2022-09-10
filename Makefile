.PHONY: build up test lint

NAME := efes

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(NAME)

build-docker:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	mkdir -p ./bin
	docker create --name efes-builder efes-base
	docker cp efes-builder:/usr/local/bin/efes - > ./bin/efes; \
	docker rm -v efes-builder

MAIN_COMPOSE := docker-compose -f ./Docker/docker-compose.yml
up:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	$(MAIN_COMPOSE) rm -fsv
	$(MAIN_COMPOSE) up --build

TEST_COMPOSE := docker-compose -f ./Docker/docker-compose-test.yml
test:
	docker build -t efes-base -f ./Docker/efes-base/Dockerfile .
	$(TEST_COMPOSE) rm -fsv
	mkdir -p ./coverage
	$(TEST_COMPOSE) up --build --exit-code-from test --abort-on-container-exit

lint:
	golangci-lint run
