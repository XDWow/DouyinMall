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
COPY --from=builder /app/internal/seckill/config/dev.yaml ./internal/seckill/config/dev.yaml
COPY --from=builder /app/internal/seckill/config/prod.yaml ./internal/seckill/config/prod.yaml
EXPOSE 8098 18098
ENTRYPOINT ["./seckill"]
CMD ["--config", "internal/seckill/config/prod.yaml"]
