.PHONY: generate fmt test

generate:
	python3 tools/gen_opcodes.py
	gofmt -w ./internal/protocol

fmt:
	gofmt -w ./cmd ./internal

test: generate fmt
	go test ./...
