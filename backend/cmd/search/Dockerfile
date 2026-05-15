FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o search ./cmd/search

FROM alpine:3.21
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/search .
COPY --from=builder /app/internal/search/config/prod.yaml ./internal/search/config/prod.yaml
EXPOSE 8093 18093
ENTRYPOINT ["./search", "--config", "internal/search/config/prod.yaml"]
