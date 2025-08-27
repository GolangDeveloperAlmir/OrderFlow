.PHONY: build run test docker swag

build:
	go build -o orderflow .

run:
	go run main.go

test:
	go test ./...

docker:
	docker-compose up --build

swag:
	swag init -g main.go -o docs
