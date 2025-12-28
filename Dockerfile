# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 复制源码
COPY go.mod ./
COPY main.go ./

# 下载依赖并编译
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o metrics-agent .

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/metrics-agent .

EXPOSE 8080

CMD ["./metrics-agent"]
