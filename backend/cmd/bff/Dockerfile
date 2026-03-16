FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bff ./cmd/bff

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/bff .
COPY --from=builder /app/internal/bff/config/dev.yaml ./internal/bff/config/dev.yaml
ENTRYPOINT ["./bff", "--config", "internal/bff/config/dev.yaml"]
