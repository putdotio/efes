.PHONY: build up test upload lint

NAME := efes

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(NAME)

up:
	docker build -t $(NAME) -f Dockerfile .
	docker-compose rm -fsv
	docker-compose up --build

test: export COMPOSE_PROJECT_NAME=$(NAME)test
test:
	docker-compose rm -fsv
	docker-compose -f docker-compose-test.yml up --build --exit-code-from test --abort-on-container-exit

upload: build
	@md5 $(NAME) > $(NAME).md5
	aws s3 cp $(NAME) s3://putio-bin
	aws s3 cp $(NAME).md5 s3://putio-bin
	@rm $(NAME) $(NAME).md5

lint:
	gometalinter --vendor -D gotype --cyclo-over=20 ./...
