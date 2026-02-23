FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY src/. .
# Мы не качаем из сети, а используем готовую папку vendor
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o producer ./cmd/app/producer/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o consumer ./cmd/app/consumer/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/producer .
COPY --from=builder /app/consumer .

COPY configs/ ./configs
COPY migrations/ ./migrations
