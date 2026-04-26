.PHONY: run build test migrate

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test ./... -v

migrate:
	go run cmd/server/main.go -migrate
