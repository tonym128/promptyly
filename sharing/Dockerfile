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
COPY sharing/ ./sharing/

# Compile the sharing server statically
RUN CGO_ENABLED=0 GOOS=linux go build -o sharing-server ./sharing

# Compile all daemon binaries for multiple OS architectures
RUN mkdir -p /app/binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/binaries/promptyly-linux-amd64 main.go sharingclient.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /app/binaries/promptyly-linux-arm64 main.go sharingclient.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o /app/binaries/promptyly-linux-arm main.go sharingclient.go
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /app/binaries/promptyly-windows-amd64.exe main.go sharingclient.go
RUN CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o /app/binaries/promptyly-windows-arm64.exe main.go sharingclient.go
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /app/binaries/promptyly-darwin-arm64 main.go sharingclient.go
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /app/binaries/promptyly-darwin-amd64 main.go sharingclient.go
RUN CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -o /app/binaries/promptyly-android-arm64 main.go sharingclient.go

# Install zip and package the browser extension
RUN apk add --no-cache zip && zip -r /app/binaries/promptyly-extension.zip browser-extension

# Install curl and conditionally download llamafile
RUN apk add --no-cache curl
ARG INCLUDE_LLAMAFILE=false
RUN if [ "$INCLUDE_LLAMAFILE" = "true" ] ; then \
      echo "📥 Downloading Qwen2.5-Coder-1.5B llamafile..." && \
      curl -L -f "https://huggingface.co/Bojun-Feng/Qwen2.5-Coder-1.5B-Instruct-GGUF-llamafile/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf" -o /app/binaries/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile && \
      chmod +x /app/binaries/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile ; \
    fi

# Run Stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/sharing-server .

# Copy pre-compiled binaries into a distribution folder to restore on container startup
COPY --from=builder /app/binaries/ /app/binaries_dist/

# Expose port
EXPOSE 6072

# Configure default directory for persistence volume
RUN mkdir -p /app/data
VOLUME /app/data

ENTRYPOINT ["./sharing-server", "-data", "/app/data"]
