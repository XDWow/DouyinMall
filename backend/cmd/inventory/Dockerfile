FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o inventory ./cmd/inventory

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/inventory .
COPY --from=builder /app/internal/inventory/config/prod.yaml ./internal/inventory/config/prod.yaml
EXPOSE 8094 19096
ENTRYPOINT ["./inventory", "--config", "internal/inventory/config/prod.yaml"]
