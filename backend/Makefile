.PHONY: gen-user build-user test docker-up docker-down docker-infra docker-clean docker-logs

# ==================== 代码生成 ====================
# 生成 user 服务的 RPC 代码
# -module: 使用 backend 的 module 路径（单 module 结构）
# -I: proto 文件的 import 搜索路径
# --gen-path: 生成代码的目标目录
gen-user:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen ./idl/user.proto

build-user:
	cd cmd/user && sh build.sh

gen-product:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen/kitex_gen ./idl/product.proto

gen-search:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen/kitex_gen ./idl/search.proto

gen-cart:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen/kitex_gen ./idl/cart.proto

gen-merchant:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen/kitex_gen ./idl/merchant.proto

gen-order:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen/kitex_gen ./idl/order.proto

# ==================== 测试 ====================
# 运行所有单元测试
test:
	go test ./internal/user/... -v

# 运行测试并查看覆盖率
test-coverage:
	go test ./internal/user/... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# 运行 search 服务的集成测试（需要 Docker）
test-integration-search:
	go test -tags=integration ./internal/search/integration/... -v

# 运行所有集成测试
test-integration:
	go test -tags=integration ./internal/... -v

# ==================== Docker Compose ====================
# 启动所有服务（基础设施 + 微服务）- 用于集成测试和生产
docker-up:
	cd .. && docker-compose up -d

# 只启动基础设施（MySQL、Redis、etcd）- 用于本地开发
docker-infra:
	cd .. && docker-compose up mysql redis etcd -d

# 停止所有服务
docker-down:
	cd .. && docker-compose down

# 停止并删除所有数据（包括数据卷）
docker-clean:
	cd .. && docker-compose down -v

# 查看所有服务日志
docker-logs:
	cd .. && docker-compose logs -f

# 查看 user-service 日志
docker-logs-user:
	cd .. && docker-compose logs -f user-service

# 重新构建并启动
docker-rebuild:
	cd .. && docker-compose up -d --build

