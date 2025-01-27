.PHONY: build up test lint

NAME := efes

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(NAME)

MAIN_COMPOSE := docker-compose -f ./Docker/docker-compose.yml
build-docker:
	$(MAIN_COMPOSE) build efes-base
	mkdir -p ./bin
	docker create --name efes-builder efes/base
	docker cp efes-builder:/usr/local/bin/efes - > ./bin/efes
	docker rm -v efes-builder

MAIN_COMPOSE := docker-compose -f ./Docker/docker-compose.yml
up:
	$(MAIN_COMPOSE) build efes-base
	$(MAIN_COMPOSE) rm -fsv
	$(MAIN_COMPOSE) up efes-tracker efes-server --build --abort-on-container-exit

TEST_COMPOSE := docker-compose -f ./Docker/docker-compose-test.yml
test:
	$(TEST_COMPOSE) rm -fsv
	mkdir -p ./coverage
	$(TEST_COMPOSE) up efes-test --build --exit-code-from efes-test --abort-on-container-exit

lint:
	golangci-lint run
