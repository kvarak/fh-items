IMAGE := fh-items
CONTEXT := .
DOCKERFILE := Dockerfile
PORT := 8080
STATICCHECK := honnef.co/go/tools/cmd/staticcheck@v0.8.2
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0

.PHONY: build test serve check fmt-check vet staticcheck vulncheck test-race js-check

# Builds the image from this directory alone - the webapp, nothing else
# from the workspace - and includes the Go toolchain so test can also run
# inside it.
build:
	docker build --build-arg GIT_SHA=$$(git rev-parse HEAD) -t $(IMAGE) -f $(DOCKERFILE) $(CONTEXT)

test: build
	docker run --rm $(IMAGE) go test ./...

# Serves the website on http://localhost:$(PORT).
serve: build
	docker run --rm -p $(PORT):8080 $(IMAGE)

# Runs every check natively (needs Go and Node); used by CI.
check: fmt-check vet staticcheck vulncheck test-race js-check

fmt-check:
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

vet:
	go vet ./...

staticcheck:
	go run $(STATICCHECK) ./...

vulncheck:
	go run $(GOVULNCHECK) ./...

test-race:
	go test -race ./...

js-check:
	node --check web/static/app.js
