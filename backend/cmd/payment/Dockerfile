FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o payment ./cmd/payment

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/payment .
COPY --from=builder /app/internal/payment/config/docker.yaml ./internal/payment/config/dev.yaml
EXPOSE 8092 8093
ENTRYPOINT ["./payment"]
