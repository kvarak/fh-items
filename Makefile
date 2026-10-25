.PHONY: test gendata run

test:
	go test ./...

gendata:
	go run ./cmd/gendata

run:
	go run ./cmd/server
