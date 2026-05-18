FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o scrapper .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    chromium \
    ca-certificates \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/scrapper .
COPY db/migrations ./db/migrations

ENV CHROME_BIN=/usr/bin/chromium

EXPOSE 50051
CMD ["./scrapper"]
