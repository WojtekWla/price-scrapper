FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./gateway


FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /app/gateway .
COPY --from=builder /app/web ./web

EXPOSE 8080
ENTRYPOINT ["./gateway"]

CMD ["-grpc-addr", "app:50051", "-http-addr", ":8080", "-web-dir", "./web"]
