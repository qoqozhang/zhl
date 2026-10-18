# ==========================================
# Stage 1: Builder
# ==========================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /app/iam-go ./cmd/main.go

# ==========================================
# Stage 2: Runtime
# ==========================================
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai \
    LANG=zh_CN.UTF-8

WORKDIR /app

COPY --from=builder /app/iam-go          /app/iam-go
COPY --from=builder /app/configs         /app/configs

EXPOSE 8080

USER nobody:nobody

ENTRYPOINT ["/app/iam-go"]