# API 集成指南（Agent / 支付）

本文说明如何将外部 Agent、脚本或服务接入 AI Pay 后端，与控制台（Next.js）使用的同一套 HTTP API 对齐。

## 1. Base URL

| 场景 | 地址 |
|------|------|
| 本地默认后端 | `http://127.0.0.1:8080` |
| 一键脚本持久化模式 | 常见为 `http://127.0.0.1:18080`（以 `frontend/.env.local` 中 `NEXT_PUBLIC_API_BASE_URL` 为准） |
| 经控制台代理 | 浏览器内请求走 `GET/POST /api/backend/*`（由 Next 注入 `X-User-Id` / `X-Tenant-Id`），**直连后端集成请勿依赖该路径** |

对外集成应直连 Go 服务端口；控制台「设置」里的 API 地址仅影响浏览器内代理。

## 2. OpenAPI

仓库根目录 [`openapi.yaml`](../openapi.yaml) 为契约定义，可用 Swagger UI / Redoc 加载，或与代码生成工具配合使用。

## 3. 控制台会话与直连后端

- 浏览器已登录时，Next.js API 代理会自动带上 `X-User-Id`、`X-Tenant-Id`（来自会话 Cookie）。
- **服务端 Agent / cron / 其他服务** 直连后端时，若业务依赖租户隔离，需按部署约定自行传入上述请求头（或由你们在 API 网关统一注入）。

部分路由（如 `/developer/api-keys`）在后端配置了 `ADMIN_BEARER_TOKEN` / 只读 Token 时，还需要 `Authorization: Bearer <token>`。

## 4. 最小闭环：注册 Agent → 开户 → 充值 → 授权 → 支付

以下字段名与 [`openapi.yaml`](../openapi.yaml) 一致；JSON 使用 camelCase（例如 `agentDid`、`didPubKey`）。

### 4.1 注册 DID 与公钥

`POST /agent/did/register`

```json
{
  "agentDid": "did:gusd:agent:example",
  "didPubKey": "<Base64 编码的 Ed25519 公钥原始 32 字节>"
}
```

### 4.2 创建账户（VA 等）

`POST /account/create`

```json
{ "agentDid": "did:gusd:agent:example" }
```

### 4.3 充值（测试流程）

`POST /fund/recharge`  

请求头：`Idempotency-Key: <唯一键>`

```json
{
  "vaAccountId": "<上一步返回的 VA 账户 ID>",
  "amount": "100"
}
```

### 4.4 设置授权规则

`POST /authorize/payment/set`

```json
{
  "agentDid": "did:gusd:agent:example",
  "singleLimit": "50",
  "dailyLimit": "200",
  "whitelist": ["m1"]
}
```

### 4.5 发起支付（x402）

`POST /payment/x402/pay`

请求头：

- `Idempotency-Key: <唯一键>`
- `X-Sign-Timestamp: <RFC3339 时间戳，需在服务端允许的窗口内>`

请求体：

```json
{
  "payerDid": "did:gusd:agent:example",
  "merchantId": "m1",
  "amount": "10",
  "signature": "<Base64 Ed25519 签名>"
}
```

**签名字符串（UTF-8）原文为：**

```text
{payerDid}|{merchantId}|{amount}|{Idempotency-Key}|{X-Sign-Timestamp}
```

与控制台 `frontend/lib/agent-signature.ts` 中 `buildPaySignPayload` 一致；须使用 **注册时同一私钥** 对原文签名。

### 4.6 查询订单状态

`GET /payment/status/query?transactionId=<支付返回的 transactionId>`

## 5. Node.js 示例

参见 [`examples/node-agent-pay/`](../examples/node-agent-pay/)：生成密钥、注册、建户、充值、授权、支付、查状态的一条龙脚本（需本地后端可访问）。

## 6. 与 OpenClaw / MCP / Skills 的关系

- 本仓库提供 **HTTP API + OpenAPI**；OpenClaw 等客户端可通过 **curl / Node / 任意语言** 调用相同接口。
- **Skills** 更适合封装「调用顺序、错误重试、幂等键生成」等产品级逻辑；建议在应用侧或独立 SDK 仓库维护 Skill，本仓库以 API 契约为准。

## 7. 幂等与频控

- 写操作请始终携带 **`Idempotency-Key`**（支付、充值等）。
- 支付需有效 **`X-Sign-Timestamp`**，避免重放。
- 响应中的 **`requestId`** 可用于排障与对账侧关联（参见控制台账单与 AI 助手上下文复制）。
