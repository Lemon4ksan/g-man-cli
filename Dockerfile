FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

COPY . .

# Build the g-mand daemon binary
RUN CGO_ENABLED=0 GOOS=linux go build -o g-mand ./cmd/g-mand

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

# Copy the compiled daemon binary from the builder stage
COPY --from=builder /app/g-mand .

# Start the daemon
CMD ["./g-mand"]
