.PHONY: \
	gen-user gen-product gen-search gen-cart gen-merchant gen-order go-payment go-checkout \
	build-user \
	test test-coverage test-integration-search test-integration \
	test-e2e-checkout test-e2e-seckill test-e2e-core \
	docker-up docker-up-core docker-infra docker-down docker-clean docker-logs docker-logs-user docker-rebuild

# ==================== Code Generation ====================
gen-user:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen ./idl/user.proto

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

go-payment:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen/kitex_gen ./idl/payment.proto

go-checkout:
	kitex -module github.com/XDWow/DouyinMall/backend -I ./idl --gen-path ./rpc_gen/kitex_gen ./idl/checkout.proto

build-user:
	cd cmd/user && sh build.sh

# ==================== Tests ====================
test:
	go test ./internal/user/... -v

test-coverage:
	go test ./internal/user/... -coverprofile=coverage.out
	go tool cover -html=coverage.out

test-integration-search:
	go test -tags=integration ./internal/search/integration/... -v

test-integration:
	go test -tags=integration ./internal/... -v

test-e2e-checkout:
	go test -tags=integration ./internal/checkout/integration -run TestCheckoutComposeEndToEnd -v

test-e2e-seckill:
	go test -tags=integration ./internal/seckill/integration -run TestSeckillComposeEndToEnd -v

test-e2e-core:
	go test -tags=integration ./internal/checkout/integration ./internal/seckill/integration -v

# ==================== Docker Compose ====================
docker-up:
	cd .. && docker compose up -d

docker-up-core:
	cd .. && docker compose up -d mysql redis etcd kafka mock-wechat-service product-service order-service inventory-service payment-service checkout-service seckill-service

docker-infra:
	cd .. && docker compose up -d mysql redis etcd

docker-down:
	cd .. && docker compose down

docker-clean:
	cd .. && docker compose down -v

docker-logs:
	cd .. && docker compose logs -f

docker-logs-user:
	cd .. && docker compose logs -f user-service

docker-rebuild:
	cd .. && docker compose up -d --build
