FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/main.go

FROM alpine:3.23

RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S -D -H -G appuser appuser

USER appuser
WORKDIR /app/
COPY --from=builder --chown=appuser:appuser /app/server ./server
EXPOSE 8080
CMD ["./server"]