APP_NAME=devops

.PHONY: build run test docker-build

build:
	GO111MODULE=on go build -o $(APP_NAME) ./

run:
	go run ./

test:
	go test ./...

docker-build:
	docker build -t $(APP_NAME):latest .

