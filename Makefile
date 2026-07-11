SWAG    := $(shell go env GOPATH)/bin/swag
MIGRATE := migrate
DB_URL  ?= postgres://appraisal:appraisal@localhost:5435/inspect_db?sslmode=disable

.PHONY: generate build run test migrate-up migrate-down up up-all down

generate:
	$(SWAG) init -g cmd/server/main.go --output api

build: generate
	go build ./...

run: generate
	go run cmd/server/main.go

test:
	go test ./...

migrate-up:
	$(MIGRATE) -path migrations -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE) -path migrations -database "$(DB_URL)" down 1

up:
	docker compose up -d

up-all:
	docker compose --profile app up -d --build

down:
	docker compose --profile app down
