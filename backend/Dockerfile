FROM golang:1.27-alpine AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Development: rebuilds and restarts the server whenever a .go file changes.
FROM base AS dev
RUN go install github.com/air-verse/air@latest
COPY . .
CMD ["air", "-c", ".air.toml"]

FROM base AS build
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.22 AS prod
COPY --from=build /server /server
EXPOSE 8080
CMD ["/server"]
