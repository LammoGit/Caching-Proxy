FROM golang:1.27.2-bookworm AS builder

ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/main ./cmd/caching-proxy

FROM debian:bookworm-slim AS final

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

COPY --from=builder /app/main .
COPY ./filter.yml .
COPY ./ca.crt .
COPY ./key.key .

EXPOSE 8080

ENTRYPOINT ["./main"]
