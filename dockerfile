FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY src/go.mod src/go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o producer .src/cmd/app/producer/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o consumer .src/cmd/app/consumer/main.go

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache curl

WORKDIR /root/
COPY --from=builder /app/producer .
COPY --from=builder /app/consumer .

COPY configs/ ./configs
COPY migrations/ ./migrations
