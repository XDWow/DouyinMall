## External Middleware Deployment

This deployment layout assumes:

- `APISIX` and the business services run on the same machine.
- `MySQL`, `Redis`, `RocketMQ`, Kafka, and `etcd` are external.
- APISIX proxies HTTP.
- Service-to-service traffic uses gRPC over etcd service discovery.

### Included routes

- `POST /api/auth/*` -> `bff-service`
- `GET|POST|PUT|DELETE /api/cart*` -> `cart-service`
- `GET /api/products*` -> `product-service`
- `GET /api/search*` -> `search-service`
- `POST /api/checkout*` -> `checkout-service`
- `GET /api/orders*` -> `order-service`
- `GET /api/seckill/activities/*` -> `seckill-service`
- `POST /api/seckill/submit` -> `seckill-service`
- `GET /api/seckill/result` -> `seckill-service`
- `POST /payment/wechat/callback` -> `payment-service`
- `POST /payment/alipay/callback` -> `payment-service`

This compose also starts the internal-only `inventory-service` and `coupon-service`, because normal checkout depends on them even though APISIX does not expose them.

`/api/agent/*` is still left out of this core compose. The current agent stack has extra MCP/LLM/Qdrant dependencies and is better treated as a separate compose/profile.

### How to use

1. Edit `deploy/.env`. `deploy/.env.example` is kept as the template.
2. The shared middleware values have already been copied into the service config files under `backend/internal/*/config/`.
3. Edit the remaining service-specific values in those code-local config files when needed. The main ones still likely to need attention are payment callback settings and search model credentials.
4. Start the service stack:

```bash
docker compose -f deploy/compose.edge.yml up -d --build
```

5. Wait for APISIX to become healthy, then initialize routes:

```bash
docker compose -f deploy/compose.edge.yml run --rm apisix-init
```

6. Traffic enters through:

```text
http://<host>:9080
```

### Notes

- APISIX config is stored in external etcd.
- The route bootstrap uses the APISIX Admin API, so the created routes are persisted in etcd.
- APISIX currently handles route control and strips spoofable identity headers such as `X-User-ID`.
- Protected upstreams still verify the `Authorization: Bearer <access_token>` token themselves by using the shared `JWT_ACCESS_SECRET`.
- `deploy/.env` is the single env file for this core compose.
- Compose now mounts the config files directly from `backend/internal/*/config/`, so the code directory is the single source of truth again.
- I copied `db.host=134.175.67.79`, `redis.addr=114.132.75.56:16379`, `rocketmq.endpoint=42.193.160.221:8081`, and `etcd.endpoints=42.193.160.221:2379` into the core services. Kafka remains only for services that still use it.
- If you later want APISIX to validate the custom BFF-issued JWT itself, add a dedicated auth service or an APISIX ext-plugin. The current repo does not have that edge-side verifier yet.
- `checkout-service` remains the business orchestrator for normal orders.
- `seckill-service` remains a direct APISIX upstream and does not pass through BFF.
- `inventory-service` and `coupon-service` are internal-only. They run inside this compose, but APISIX does not expose them.
- `search` AI endpoints need valid LLM and embedding credentials in `backend/internal/search/config/dev.yaml`.
- `payment` callback URLs in `backend/internal/payment/config/dev.yaml` should point to the public APISIX address, not the container name.
- `agent-service` is not part of this core compose yet. Its current config still contains MCP endpoints like `localhost:1909x`, which are not container-ready without another round of adjustment.
