# Build context is fh-items/ itself; the image only sees the webapp.
FROM golang:1.23-alpine

WORKDIR /src
COPY . .
# .git is not in the build context, so the commit SHA comes in as a build arg.
ARG GIT_SHA=""
RUN go build -ldflags "-X main.commit=${GIT_SHA}" -o /out/server ./cmd/server

EXPOSE 8080
CMD ["/out/server", "-addr", ":8080"]
