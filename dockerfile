FROM golang:1.25-alpine

# Install build dependencies, ping utilities, and file capability management tools
RUN apk update && apk add --no-cache iputils libcap

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build the Go application binary
RUN go build -o nms-middleware .

# Grant the raw network capabilities to the compiled Go binary explicitly
RUN setcap cap_net_raw=+ep nms-middleware

# Execute the binary
CMD ["./nms-middleware", "-db-dsn", "postgres://postgres:yoursecurepassword@timescaledb:5432/nms_db?sslmode=disable", "-env", "development"]