NAME := efes

build:
	GOOS=linux GOARCH=amd64 go build -o $(NAME)
	docker build -t efes-server:latest -f server/Dockerfile .
	docker build -t efes-tracker:latest -f tracker/Dockerfile .
upload: build
	@md5 $(NAME) > $(NAME).md5
	aws s3 cp $(NAME) s3://putio-bin
	aws s3 cp $(NAME).md5 s3://putio-bin
	@rm $(NAME) $(NAME).md5

lint:
	gometalinter --vendor -D gotype ./...
