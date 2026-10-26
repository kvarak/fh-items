IMAGE := fh-items
CONTEXT := .
DOCKERFILE := Dockerfile
PORT := 8080

.PHONY: build test gendata serve

# Builds the image from this directory alone - the webapp, nothing else
# from the workspace - and includes the Go toolchain so test/gendata can
# also run inside it.
build:
	docker build -t $(IMAGE) -f $(DOCKERFILE) $(CONTEXT)

test: build
	docker run --rm $(IMAGE) go test ./...

# Regenerates internal/catalog/data/catalog.gen.json from the immutable
# source. The image itself has no knowledge of the workspace outside
# fh-items/; the source file and output dir are bind mounted in just for
# this one-off run.
gendata: build
	docker run --rm \
		-v $(CURDIR)/../.github/docs:/.github/docs:ro \
		-v $(CURDIR)/internal/catalog/data:/src/internal/catalog/data \
		$(IMAGE) go run ./cmd/gendata

# Serves the website on http://localhost:$(PORT).
serve: build
	docker run --rm -p $(PORT):8080 $(IMAGE)
