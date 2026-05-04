# Build Stage
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o orthodoxpilgrimage ./cmd/server

FROM litestream/litestream:0.5-scratch AS litestream
FROM busybox:1.37-uclibc AS busybox

# Final Stage
FROM gcr.io/distroless/static-debian12
COPY --from=busybox /bin/busybox /bin/sh
COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream
COPY scripts/run.sh /scripts/run.sh
COPY litestream.yml /etc/litestream.yml
COPY --from=builder /app/orthodoxpilgrimage .

EXPOSE 8080
CMD [ "/scripts/run.sh" ]
