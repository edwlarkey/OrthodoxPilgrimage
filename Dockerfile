# Build Stage
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build binary
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o orthodoxpilgrimage ./cmd/server

# Final Stage
FROM gcr.io/distroless/static-debian12
WORKDIR /root/
COPY --from=builder /app/orthodoxpilgrimage .

EXPOSE 8080
CMD ["./orthodoxpilgrimage"]
