.PHONY: build test proto run lint
build:
	go build -o bin/oikos ./cmd/oikos
test:
	go test ./...
proto:
	bash scripts/gen-proto.sh
run:
	go run ./cmd/oikos
lint:
	golangci-lint run ./...
