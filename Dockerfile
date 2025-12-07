# 前端构建阶段
FROM node:18-alpine AS frontend-builder
WORKDIR /frontend
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Go 后端构建阶段
FROM golang:alpine AS builder
WORKDIR /build
ADD go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# 从前端构建阶段复制构建产物
COPY --from=frontend-builder /frontend/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

# 最终运行阶段
FROM alpine:latest
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*
WORKDIR /app
COPY --from=builder /build/app .
COPY --from=builder /build/web/dist ./web/dist
EXPOSE 8080
CMD ["./app"]
