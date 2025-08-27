.PHONY: build run test docker swag

build:
        go build -o orderflow ./cmd/api

run:
        go run ./cmd/api

test:
	go test ./...

docker:
	docker-compose up --build

swag:
        swag init -g cmd/api/main.go -o docs
