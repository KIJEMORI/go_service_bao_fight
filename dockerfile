FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o producer ./cmd/app/producer/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o consumer ./cmd/app/consumer/main.go

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache curl

WORKDIR /root/
COPY --from=builder /app/producer .
COPY --from=builder /app/consumer .

COPY configs/ ./configs
COPY migrations/ ./migrations
