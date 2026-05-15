FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o coupon ./cmd/coupon

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/coupon .
COPY --from=builder /app/internal/coupon/config/prod.yaml ./internal/coupon/config/prod.yaml
EXPOSE 8097
ENTRYPOINT ["./coupon", "--config", "internal/coupon/config/prod.yaml"]
