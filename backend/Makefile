SHELL := /bin/bash

.PHONY: up down run test bench fmt tidy migrate health ready

up:
	docker compose up -d

down:
	docker compose down

run:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	GOPROXY=https://goproxy.cn,direct go run ./cmd/server

test:
	GOPROXY=https://goproxy.cn,direct go test ./...

bench:
	GOPROXY=https://goproxy.cn,direct go test -run '^$$' -bench BenchmarkPayInMemory -benchmem ./internal/service

fmt:
	gofmt -w ./cmd ./internal

tidy:
	GOPROXY=https://goproxy.cn,direct go mod tidy

migrate:
	mysql -uroot -proot ai_pay < migrations/001_init.sql
	mysql -uroot -proot ai_pay < migrations/002_add_fee_and_recharge_log.sql

health:
	curl -s http://127.0.0.1:8080/health

ready:
	curl -s http://127.0.0.1:8080/ready
