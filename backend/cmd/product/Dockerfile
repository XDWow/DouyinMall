FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o product ./cmd/product

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/product .
COPY --from=builder /app/internal/product/config/prod.yaml ./internal/product/config/prod.yaml
EXPOSE 8096 18096 19095
ENTRYPOINT ["./product", "--config", "internal/product/config/prod.yaml"]
