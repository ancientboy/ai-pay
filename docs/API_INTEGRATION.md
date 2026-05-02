# API 集成指南（Agent / 支付）

本文说明如何将外部 Agent、脚本或服务接入 **AgentTrust Pay（可信付）** 后端，与控制台（Next.js）使用的同一套 HTTP API 对齐。

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

## 8. 给小白：钱从哪来、支付能力是什么

可以把它想成 **「带 VA 钱包的 Agent」**：

1. **先有一个账户**  
   注册 `agentDid` + 公钥后，再 `POST /account/create`，你会得到 **VA 账户 ID**（类似钱包账号）。

2. **账户里要有余额，才能付**  
   扣款时系统会检查 **VA 可用余额**；不足则返回 `PAY-003`（agent va insufficient balance）。  
   在联调 / MVP 里，余额通常来自 **`POST /fund/recharge`**（模拟入金），或你们在生产中接好的真实入金通道。  
   **注意**：给组织买 **订阅/套餐**（Billing）是付给平台的产品费用；给 **VA 充值** 才是给 Agent 的「可支付余额」。两者不要混用。

3. **「有支付能力」在系统里 = 四件事同时满足**  
   - 已配置 **授权规则**（`POST /authorize/payment/set`），且状态为 **ACTIVE**；  
   - 目标 **merchantId 在白名单**里；  
   - 金额不超过 **单笔/日限额**；  
   - **VA 余额 ≥ 支付金额**。

   **merchantId 是什么、要不要商户入驻？**  
   它是本系统授权规则里使用的**收款方标识字符串**，由租户与对方业务约定（例如 `partner_api_x`）。当前 MVP **不要求**收款方在本平台单独注册；把约定好的 ID 加入白名单即可。  
   这与「互联网上任意接受稳定币的商家都能被扣款」不同：本仓库实现的是**平台内清算与记账**；链上稳定币结算、外部生态的 x402 互通需要额外的网关或商户接入（可后续迭代）。

4. **扣款时 Agent 要证明「我是本人」**  
   用注册时同一对密钥里的 **私钥**，对固定格式的字符串做 **Ed25519 签名**，随 `POST /payment/x402/pay` 提交。  
   外部程序、定时任务、OpenClaw 等都是 **HTTP 调用同一套 API**，与浏览器控制台一致。

5. **扣款之后**  
   得到 `transactionId`，用 `GET /payment/status/query` 或控制台的 **交易** 页查看；若走异步清分，可能先为 `SETTLING` 再变 `SETTLED`（视通道与配置而定）。
