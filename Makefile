IMAGE := fh-items
CONTEXT := .
DOCKERFILE := Dockerfile
PORT := 8080

.PHONY: build test serve

# Builds the image from this directory alone - the webapp, nothing else
# from the workspace - and includes the Go toolchain so test can also run
# inside it.
build:
	docker build -t $(IMAGE) -f $(DOCKERFILE) $(CONTEXT)

test: build
	docker run --rm $(IMAGE) go test ./...

# Serves the website on http://localhost:$(PORT).
serve: build
	docker run --rm -p $(PORT):8080 $(IMAGE)
