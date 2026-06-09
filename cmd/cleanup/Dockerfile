# cmd/cleanup/Dockerfile
FROM golang:1.25 AS builder

WORKDIR /app

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the cleanup binary statically
RUN cd cmd/cleanup && CGO_ENABLED=0 GOOS=linux go build -o /cleanup

# Final stage: use scratch for a minimal image
FROM scratch

WORKDIR /app
COPY --from=builder /cleanup .

# Run the cleanup worker by default
CMD ["./cleanup"]
