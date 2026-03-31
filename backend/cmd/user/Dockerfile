FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o user ./cmd/user

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/user .
COPY --from=builder /app/internal/user/config/dev.yaml ./internal/user/config/dev.yaml
EXPOSE 8091
ENTRYPOINT ["./user", "--config", "internal/user/config/dev.yaml"]
