# AI Pay Backend MVP

Go 实现的 MVP 后端，覆盖文档中第一阶段核心接口：

- `POST /agent/did/register`
- `POST /account/create`
- `POST /fund/recharge`
- `POST /authorize/payment/set`
- `POST /payment/x402/pay`
- `GET /payment/status/query`
- `GET /account/balance/query`
- `GET /account/ledger/query`

## 安全基线（M2）

- 请求体字段做基础校验（必填、正数金额、白名单非空）
- `POST /payment/x402/pay` 强制要求：
  - `Idempotency-Key` 请求头
  - `X-Sign-Timestamp` 请求头（RFC3339，默认 5 分钟有效期）
- 基础限流：
  - 按 IP 每分钟限制
  - 按 Agent DID 每分钟限制
- 支付日志脱敏输出（不记录签名原文）

## 可观测与补偿（M3）

- 每个请求自动注入并回传 `X-Request-Id`
- 持久化模式下自动启动定时任务：
  - 状态补偿（每 1 分钟）：将超时 `SETTLING` 订单标记为 `FAILED`
  - 日对账（每 5 分钟）：校验 `充值-支付` 与账户余额一致性
- 对账异常与任务失败会输出 `[ALERT]` 日志

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

如果你在本地已经初始化过数据库，请手动执行：

```bash
mysql -uroot -proot ai_pay < migrations/002_add_fee_and_recharge_log.sql
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

## 说明

- 已支持 MySQL + Redis 持久化（推荐）。
- 保留内存模式，便于快速联调。
