FROM golang:1.25-alpine

# Install build dependencies and raw socket permissions utilities via apk
RUN apk update && apk add --no-cache iputils

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build the Go application binary
RUN go build -o nms-middleware .

# Execute the binary
CMD ["./nms-middleware", "-db-dsn", "postgres://postgres:yoursecurepassword@timescaledb:5432/nms_db?sslmode=disable", "-env", "development"]