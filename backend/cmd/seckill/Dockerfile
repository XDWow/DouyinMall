FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o seckill ./cmd/seckill

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/seckill .
COPY --from=builder /app/internal/seckill/config/docker.yaml ./internal/seckill/config/dev.yaml
EXPOSE 8098
ENTRYPOINT ["./seckill"]
