FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o aftersale ./cmd/aftersale

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/aftersale .
COPY --from=builder /app/internal/aftersale/config/prod.yaml ./internal/aftersale/config/prod.yaml
EXPOSE 8097 19097
ENTRYPOINT ["./aftersale", "--config", "internal/aftersale/config/prod.yaml"]
