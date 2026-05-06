## Local Seckill Smoke

Start the local stack:

```powershell
./scripts/local-start-seckill-stack.ps1 -ConfigMode dev -RocketMQEndpoint "<rocketmq-endpoint>"
```

Stop it:

```powershell
./scripts/local-stop-seckill-stack.ps1
```

Service logs are written under:

```text
.local/seckill-stack/logs/
```

Suggested smoke order:

1. Start `order`, `inventory`, `coupon`, `cart`, `seckill`.
2. Create one seckill activity.
3. Submit one seckill request and keep the returned `requestNo`.
4. Poll seckill result until it becomes `SUCCESS` or `FAILED`.
5. If it is `SUCCESS`, verify order creation with the returned `orderId`.
6. Repeat with the same user to confirm duplicate protection.
7. Repeat after stock is exhausted to confirm out-of-stock handling.
