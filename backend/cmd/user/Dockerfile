# 构建阶段
FROM golang:1.24-alpine AS builder

WORKDIR /build

# 安装必要的工具
RUN apk add --no-cache git make

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
WORKDIR /build/cmd/user
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o /app/user-service .

# 运行阶段
FROM alpine:latest

WORKDIR /app

# 安装 CA 证书（用于 HTTPS 请求）和时区数据
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 从构建阶段复制二进制文件
COPY --from=builder /app/user-service .

# 复制配置文件（如果有）
COPY --from=builder /build/internal/user/config/dev.yaml ./config/

# 暴露端口
EXPOSE 8081

# 运行服务
CMD ["./user-service"]

