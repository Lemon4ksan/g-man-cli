FROM golang:1.26-alpine AS builder

# Install git and ca-certificates to fetch private go modules
RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the gmanctl client binary
RUN CGO_ENABLED=0 GOOS=linux go build -o gmanctl ./cmd/gmanctl

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

# Copy the compiled client binary
COPY --from=builder /app/gmanctl .

# Default entrypoint allows passing commands directly (e.g., status, shell)
ENTRYPOINT ["./gmanctl"]
