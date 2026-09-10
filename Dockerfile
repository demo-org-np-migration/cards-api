# syntax=docker/dockerfile:1
FROM golang:1.26 AS build
WORKDIR /src

# cauri-go-kit isn't on the public module proxy checksum db yet.
ENV GOPROXY=direct
ENV GONOSUMDB=github.com/demo-org-np-migration
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/cards-api ./cmd/cards-api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/cards-api ./cards-api
# Migrations travel in the image so the process can run them against
# `cards` at boot (golang-migrate's file source, see internal/store/migrate.go).
COPY migrations ./migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/cards-api"]
