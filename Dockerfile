# Build context is fh-items/ itself; the image only sees the webapp.
FROM golang:1.23-alpine

WORKDIR /src
COPY . .
RUN go build -o /out/server ./cmd/server

EXPOSE 8080
CMD ["/out/server", "-addr", ":8080"]
