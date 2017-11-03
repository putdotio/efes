.PHONY: build up test upload lint

NAME := efes

build:
	GOOS=linux GOARCH=amd64 go build -o $(NAME)

up:
	docker build -t $(NAME) -f Dockerfile .
	docker-compose rm -f
	docker-compose up --build

test:
	docker-compose -p efestest rm -f
	docker-compose -p efestest -f docker-compose-test.yml up --build --exit-code-from test

upload: build
	@md5 $(NAME) > $(NAME).md5
	aws s3 cp $(NAME) s3://putio-bin
	aws s3 cp $(NAME).md5 s3://putio-bin
	@rm $(NAME) $(NAME).md5

lint:
	gometalinter --vendor -D gotype ./...
