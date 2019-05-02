.PHONY: build up test lint

NAME := efes

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(NAME)

up:
	docker build -t $(NAME) -f Dockerfile .
	docker-compose rm -fsv
	docker-compose up --build

test:
	docker-compose rm -fsv
	docker-compose -f docker-compose-test.yml up --build --exit-code-from test --abort-on-container-exit

lint:
	golangci-lint run
