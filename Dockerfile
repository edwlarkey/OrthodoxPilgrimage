# Build Stage
FROM golang:1.26-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o orthodoxpilgrimage ./cmd/server

FROM litestream/litestream:0.5 AS litestream

# Final Stage
FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    imagemagick \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream
COPY scripts/run.sh /scripts/run.sh
COPY litestream.yml /etc/litestream.yml
COPY --from=builder /app/orthodoxpilgrimage .

EXPOSE 8080
CMD [ "/scripts/run.sh" ]
