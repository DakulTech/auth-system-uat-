# cmd/worker/Dockerfile
FROM golang:1.25 AS builder

WORKDIR /app

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the worker binary statically
RUN cd cmd/worker && CGO_ENABLED=0 GOOS=linux go build -o /worker

# Final stage: use scratch for a truly minimal image
FROM scratch

WORKDIR /app
COPY --from=builder /worker .

# Run the worker by default
CMD ["./worker"]
