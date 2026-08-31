.PHONY: run build test tidy

run:
	go run ./cmd/relic

build:
	go build -o bin/relic ./cmd/relic

test:
	go test ./... -race

tidy:
	go mod tidy