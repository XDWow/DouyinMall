FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o agent ./cmd/agent

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/agent .
COPY --from=builder /app/internal/agent/config/dev.yaml ./internal/agent/config/dev.yaml
COPY --from=builder /app/internal/agent/config/prod.yaml ./internal/agent/config/prod.yaml
EXPOSE 8087 18087
ENTRYPOINT ["./agent", "--config", "internal/agent/config/prod.yaml"]


