APP_NAME := gopherai-resume

.PHONY: tidy build run

tidy:
	go mod tidy

build:
	go build -o bin/$(APP_NAME) ./cmd/server

run:
	go run ./cmd/server
