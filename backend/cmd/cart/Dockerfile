FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o cart ./cmd/cart

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/cart .
COPY --from=builder /app/internal/cart/config/prod.yaml ./internal/cart/config/prod.yaml
EXPOSE 8099 18099 19094
ENTRYPOINT ["./cart", "--config", "internal/cart/config/prod.yaml"]
