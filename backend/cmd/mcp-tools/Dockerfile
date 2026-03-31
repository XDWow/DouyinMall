FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-tools ./cmd/mcp-tools

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/mcp-tools .
COPY --from=builder /app/internal/mcptools/config/dev.yaml ./internal/mcptools/config/dev.yaml
EXPOSE 9090
ENTRYPOINT ["./mcp-tools", "--config", "internal/mcptools/config/dev.yaml"]
