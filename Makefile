.PHONY: generate build test verify-generated

generate:
	go generate ./...

build: generate
	go build ./...

test: generate
	go test ./...

verify-generated:
	./scripts/verify-generated.sh
