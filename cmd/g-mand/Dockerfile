FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

ENV GOPRIVATE=github.com/vlhltf/*

COPY go.mod go.sum ./

RUN --mount=type=secret,id=github_token,target=/run/secrets/github_token \
    git config --global url."https://$(cat /run/secrets/github_token):x-oauth-basic@github.com/".insteadOf "https://github.com/" && \
    go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o g-mand ./cmd/g-mand

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 gman && \
    adduser -u 1000 -G gman -s /bin/sh -D gman

WORKDIR /app

COPY --from=builder /app/g-mand .

RUN mkdir -p /app/data && chown -R gman:gman /app

USER gman

CMD ["./g-mand"]
