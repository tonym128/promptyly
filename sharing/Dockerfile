# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module manifest and source files
COPY go.mod ./
COPY sharing/ ./sharing/

# Compile the sharing server statically
RUN CGO_ENABLED=0 GOOS=linux go build -o sharing-server ./sharing

# Run Stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/sharing-server .

# Expose port
EXPOSE 6072

# Configure default directory for persistence volume
RUN mkdir -p /app/data
VOLUME /app/data

ENTRYPOINT ["./sharing-server", "-data", "/app/data"]
