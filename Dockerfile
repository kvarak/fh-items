# Build context is fh-items/ itself - this image only ever sees the
# webapp, never the rest of the workspace. The immutable source data at
# ../.github/docs/items.json is only needed by `make gendata`, which bind
# mounts it in at run time instead (see fh-items/Makefile).
FROM golang:1.23-alpine

WORKDIR /src
COPY . .
RUN go build -o /out/server ./cmd/server

EXPOSE 8080
CMD ["/out/server", "-addr", ":8080"]
