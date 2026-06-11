# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module manifest and source files
COPY go.mod ./
COPY main.go ./
COPY sharingclient.go ./
COPY config/ ./config/
COPY app/ ./app/
COPY server/ ./server/
COPY git/ ./git/
COPY agent/ ./agent/
COPY history/ ./history/
COPY urlscheme/ ./urlscheme/

# Compile the local developer daemon
RUN CGO_ENABLED=0 GOOS=linux go build -o promptyly main.go sharingclient.go

# Run Stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates git

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/promptyly .

# Expose local server daemon port
EXPOSE 6071

# Configure volumes for settings and app projects
VOLUME ["/root/.config/promptyly", "/root/promptyly-apps"]

ENTRYPOINT ["./promptyly", "serve"]
