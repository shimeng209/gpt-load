FROM golang:alpine AS builder

ARG VERSION=1.0.0
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOCACHE=/root/.cache/go-build \
    GOMODCACHE=/go/pkg/mod

WORKDIR /build

# 先复制 go.mod 和 go.sum 以利用 Docker 缓存
ADD go.mod go.sum ./
RUN go mod download && go mod verify

# 复制源码 (排除不必要的大文件)
COPY . .
# 复制预构建的前端资源
COPY ./web/dist ./web/dist

# 使用并行构建和优化的编译参数
RUN go build -ldflags "-s -w -X gpt-load/internal/version.Version=${VERSION}" -o gpt-load


FROM alpine:latest

# 安装运行时依赖时使用缓存
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# 复制二进制文件并设置最小权限
COPY --from=builder /build/gpt-load .
RUN chmod +x gpt-load

# 创建非 root 用户（可选的安全增强）
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup
USER appuser

EXPOSE 3001
ENTRYPOINT ["./gpt-load"]
