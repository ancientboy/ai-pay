# AI Pay Backend MVP

Go 实现的 MVP 后端，覆盖文档中第一阶段核心接口：

- `POST /agent/did/register`
- `POST /agent/did/verify`
- `POST /agent/did/update`
- `POST /account/create`
- `GET /agent/list`
- `POST /fund/recharge`（支持 `vaAccountId` 或 `vaCardNo`）
- `GET /fund/recharge/list`
- `POST /authorize/payment/set`
- `POST /authorize/payment/update`
- `POST /authorize/freeze`
- `POST /authorize/activate`
- `POST /payment/x402/pay`
- `POST /payment/status/callback`
- `POST /payment/unfreeze`
- `POST /payment/refund`
- `GET /payment/status/query`
- `GET /account/balance/query`
- `GET /account/ledger/query`
- `GET /account/interest/query`
- `POST /account/va/topup/config`
- `GET /account/va/topup/config`
- `POST /account/va/transfer`
- `GET /account/va/transfer/list`
- `GET /metrics/overview`

**可选特性开关（未开启时对应路由返回 404）：**

- `FEATURE_M6_FUNDS`：`PAY-011`
- `FEATURE_M7_CARD_RISK`：`PAY-012`
- `FEATURE_M8_SELF_HOSTED`：`PAY-013`

## 里程碑 M6（资金与支付扩展）

在 `FEATURE_M6_FUNDS=true` 时额外提供：

- `POST /fund/transfer`、`POST /fund/withdraw`
- `POST /payment/debit/preview`、`POST /payment/x402/check`、`POST /payment/x402/transfer`、`POST /payment/refund/apply`

持久化需迁移 `012_add_m6_fund_and_preview.sql`。

## 里程碑 M7（沙箱虚拟卡 + 规则风控 + 当事人 KYC 占位）

在 `FEATURE_M7_CARD_RISK=true` 时额外提供（**沙箱**，不接真实清算）：`POST /payment/card/*`、`POST /risk/*`、`GET /risk/audit/query`。持久化需 `013_add_m7_card_kyc_audit.sql`。

## 里程碑 M8（自托管钱包 + 授权会话 + 支付签名两步）

在 `FEATURE_M8_SELF_HOSTED=true` 时额外提供：

- `POST /wallet/bind`、`POST /wallet/unbind`（链上地址绑定占位）
- `POST /authorize/session/create`、`POST /authorize/session/revoke`
- `POST /payment/sign/request`（返回 `signId`）、`POST /payment/sign/submit`（复用与 `x402/pay` 相同的签名载荷）

持久化需迁移 `014_add_m8_self_host.sql`。

## 安全基线（M2）

- 请求体字段做基础校验（必填、正数金额、白名单非空）
- `POST /payment/x402/pay` 强制要求：
  - `Idempotency-Key` 请求头
  - `X-Sign-Timestamp` 请求头（RFC3339，默认 5 分钟有效期）
  - DID Ed25519 签名（签名串：`payerDid|merchantId|amount|idempotencyKey|signTimestamp`）
- `POST /fund/recharge` 强制要求：
  - `Idempotency-Key` 请求头
  - 请求体携带 `vaAccountId` 或 `vaCardNo` 任一标识
- 基础限流：
  - 按 IP 每分钟限制
  - 按 Agent DID 每分钟限制
  - 持久化模式自动启用 Redis 限流（多实例共享）
- 支付日志脱敏输出（不记录签名原文）
- 可选敏感接口鉴权（建议生产开启）：
  - `ADMIN_BEARER_TOKEN`：管理员令牌；`developer/*` 写接口与 `/payment/unfreeze`、`/payment/refund` 需要管理员令牌
  - `READONLY_BEARER_TOKEN`：只读令牌；可访问 `GET /developer/api-keys`、`GET /developer/webhooks`
  - `CALLBACK_TOKEN`：开启后 `/payment/status/callback` 需：
    - `X-Callback-Token: <token>`
    - `X-Callback-Timestamp: <RFC3339>`
    - `X-Callback-Nonce: <unique nonce>`（10 分钟窗口内不可复用，防重放）
    - `X-Callback-Idempotency-Key: <retry-stable key>`（同 key 重试会返回幂等成功，负载变化会冲突）
  - `CALLBACK_SIGNING_SECRET`：开启后额外要求：
    - `X-Callback-Signature-Version: v1`
    - `X-Callback-Signature`，算法：
      - `hex(HMAC_SHA256(secret, transactionId|status|timestamp|nonce|idempotencyKey))`

- 开发者安全能力：
  - API Key 列表接口仅返回掩码值；完整 key 仅在创建时返回一次
  - 服务端与数据库仅保存 API Key 哈希，不落明文
  - Webhook URL 拦截 `localhost`、私网/回环/链路本地地址以及 `.local/.internal` 域名，降低 SSRF 风险
  - Webhook 出站签名（可选）：配置 `WEBHOOK_SIGNING_SECRET` 后，投递附带：
    - `X-Webhook-Timestamp` / `X-Webhook-Nonce` / `X-Webhook-Attempt`
    - `X-Webhook-Signature-Version: v1`
    - `X-Webhook-Signature: hex(HMAC_SHA256(secret, deliveryId|event|timestamp|nonce|attempt|rawBody))`

## 可观测与补偿（M3）

- 每个请求自动注入并回传 `X-Request-Id`
- 持久化模式下自动启动定时任务：
  - 状态补偿（每 1 分钟）：将超时 `SETTLING` 订单标记为 `FAILED`
  - 日对账（每 5 分钟）：校验 `充值-支付` 与账户余额一致性
  - Webhook 投递执行器（每 15 秒）：处理 `payment.settled` / `payment.refunded` 事件，失败自动重试并进入死信
- 对账异常与任务失败会输出 `[ALERT]` 日志
- 新增开发者投递运维接口：
  - `GET /developer/webhook-deliveries`（支持 `status/event/webhookId/limit/offset`）
  - `GET /developer/webhook-deliveries/stats`（返回 `pending/retrying/sent/dead/total` 聚合）
  - `POST /developer/webhook-deliveries/replay`（按 `id` 重放）
  - `GET /developer/audit-logs`（支持 `action/resource/limit/offset`）
  - `GET /developer/risk-config`、`POST /developer/risk-config`
  - `GET /developer/channel-routes`、`POST /developer/channel-routes`、`DELETE /developer/channel-routes`

## 资金页运营化（Phase 4）

- `GET /account/va/transfer/list` 支持以下查询参数：
  - `accountId`：按账户过滤（转入或转出任一命中）
  - `status`：按状态过滤（`SETTLED` / `FAILED`）
  - `startTime` / `endTime`：按创建时间窗口过滤（RFC3339）
  - `limit` / `offset`：分页
- 推荐前端查询模式：
  - 列表页使用 `limit=10~20` + `offset` 做翻页
  - 导出 CSV 时沿用同一筛选参数，确保“所见即所得”

## 联调与发布（M4）

- 全链路集成测试：`internal/http/integration_flow_test.go`
- 灰度与回滚手册：`RELEASE_RUNBOOK.md`
- 一键回滚脚本：`scripts/rollback.sh`
- 健康检查接口：`GET /health`
- 就绪检查接口：`GET /ready`
- 容量基线（内存模式）：

```bash
go test -run '^$' -bench BenchmarkPayInMemory -benchmem ./internal/service
```

## 本地依赖（MySQL + Redis）

```bash
cd backend
docker compose up -d
```

初始化表结构会自动执行 `migrations/001_init.sql`。
新增字段与充值流水由 `migrations/002_add_fee_and_recharge_log.sql` 提供。  
VA 卡号字段由 `migrations/003_add_va_card_no.sql` 提供。  
冻结账务能力（`frozen_balance` + `account_hold`）由 `migrations/004_add_account_hold.sql` 提供。
DID 公钥字段由 `migrations/005_add_agent_did_pub_key.sql` 提供。
订单冻结关联字段（`pay_order.hold_id`）由 `migrations/006_add_pay_order_hold_id.sql` 提供。  
开发者资源表（API Key / Webhook）由 `migrations/007_add_developer_resources.sql` 提供。
Webhook 投递任务表（重试 + 死信）由 `migrations/008_add_webhook_delivery_task.sql` 提供。
VA 自动充值配置与 VA 转账流水表由 `migrations/009_add_va_topup_and_transfer.sql` 提供。
审计日志表由 `migrations/010_add_audit_log.sql` 提供。
风控配置与渠道路由表由 `migrations/011_add_risk_and_channel_route.sql` 提供。
默认 docker 映射端口为 `3307 -> 3306`，避免与本机已有 MySQL 冲突。

如果你在本地已经初始化过数据库，请手动执行：

```bash
mysql -uroot -proot ai_pay < migrations/002_add_fee_and_recharge_log.sql
mysql -uroot -proot ai_pay < migrations/003_add_va_card_no.sql
mysql -uroot -proot ai_pay < migrations/004_add_account_hold.sql
mysql -uroot -proot ai_pay < migrations/005_add_agent_did_pub_key.sql
mysql -uroot -proot ai_pay < migrations/006_add_pay_order_hold_id.sql
mysql -uroot -proot ai_pay < migrations/007_add_developer_resources.sql
mysql -uroot -proot ai_pay < migrations/008_add_webhook_delivery_task.sql
mysql -uroot -proot ai_pay < migrations/009_add_va_topup_and_transfer.sql
mysql -uroot -proot ai_pay < migrations/010_add_audit_log.sql
mysql -uroot -proot ai_pay < migrations/011_add_risk_and_channel_route.sql
```

## 运行

```bash
cd backend
cp .env.example .env
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
```

默认端口 `8080`，可用环境变量 `PORT` 覆盖。  
若未配置 `MYSQL_DSN` 与 `REDIS_ADDR`，服务会自动回退到内存模式。  
服务已启用基础生产超时与优雅停机（`ReadHeaderTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout` + `SIGTERM` 优雅关闭）。

`GET /ready` 在持久化模式会执行 MySQL 与 Redis 探活，依赖异常时返回 `503` 与 `ready=false`。

也可使用 `Makefile` 快速执行：

```bash
make up
make run
make test
```

## 测试

```bash
cd backend
go test ./...
```

持久化链路集成测试会读取环境变量 `MYSQL_DSN` / `REDIS_ADDR`；若未配置会自动 skip。

端到端冒烟（需前端已启动在 `:3000`）：

```bash
cd backend
chmod +x scripts/smoke_test.sh
./scripts/smoke_test.sh
```

## 说明

- 已支持 MySQL + Redis 持久化（推荐）。
- 保留内存模式，便于快速联调。
