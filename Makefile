.PHONY: build run test docker swag certs

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

certs:
	mkdir -p certs
	openssl req -x509 -newkey rsa:2048 -nodes -keyout certs/server.key -out certs/server.crt -days 365 -subj "/CN=localhost"
