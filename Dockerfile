# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./cmd/app

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
WORKDIR /app

COPY --from=builder /out/app /app/app

EXPOSE 50051
ENTRYPOINT ["/app/app"]
