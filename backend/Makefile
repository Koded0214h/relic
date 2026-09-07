.PHONY: run build test tidy bench

run:
	go run ./cmd/relic

bench:
	go run ./cmd/relic-bench

build:
	go build -o bin/relic ./cmd/relic

test:
	go test ./... -race

tidy:
	go mod tidy