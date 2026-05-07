FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/api

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/server .
COPY migrations/ ./migrations/
ENTRYPOINT ["./server"]
