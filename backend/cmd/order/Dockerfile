FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o order ./cmd/order

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/order .
COPY --from=builder /app/internal/order/config/dev.yaml ./internal/order/config/dev.yaml
COPY --from=builder /app/internal/order/config/prod.yaml ./internal/order/config/prod.yaml
EXPOSE 8095 18095
ENTRYPOINT ["./order", "--config", "internal/order/config/prod.yaml"]
