FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o checkout ./cmd/checkout

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/checkout .
COPY --from=builder /app/internal/checkout/config/prod.yaml ./internal/checkout/config/prod.yaml
EXPOSE 8086 18086
ENTRYPOINT ["./checkout", "--config", "internal/checkout/config/prod.yaml"]
