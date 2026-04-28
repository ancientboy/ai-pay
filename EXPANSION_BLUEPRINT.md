# AI Pay 后续扩展蓝图（PR6）

> 目标：在不破坏当前 MVP 稳定性的前提下，把“文档中已规划但未开发”的能力拆成可并行、可验收、可回滚的增量里程碑。

## 1. 当前边界（已完成）

- 当前后端聚焦 MVP 支付闭环：开户、充值、授权、x402 支付、状态查询、余额/流水、回调、解冻、退款、概览指标。
- 前端聚焦控制台 MVP：登录、Agents、Authorize、Recharge、Transactions、Developer、Settings。
- 已具备基础自动化：后端单测 + 集成测试、前端 lint、联调 smoke 脚本。

## 2. 未开发能力分组（来源于产品总文档）

### Group A：身份与授权增强

- `/agent/did/verify`
- `/agent/did/update`
- `/authorize/payment/update`
- `/authorize/freeze`

### Group B：资金与账户扩展

- `/account/interest/query`
- `/fund/withdraw`
- `/fund/transfer`
- `/account/va/transfer`
- `/account/va/topup/config`

### Group C：支付场景扩展

- `/payment/x402/check`
- `/payment/x402/transfer`
- `/payment/debit/preview`
- `/payment/refund/apply`

### Group D：卡支付与风控合规

- `/payment/card/apply`
- `/payment/card/pay`
- `/payment/card/manage`
- `/risk/transaction/check`
- `/risk/kyc/verify`
- `/risk/audit/query`

### Group E：自托管与会话签名

- `/wallet/bind`
- `/wallet/unbind`
- `/authorize/session/create`
- `/authorize/session/revoke`
- `/payment/sign/request`
- `/payment/sign/submit`

## 3. 推荐里程碑（按风险与收益排序）

## M5（低风险高收益）：授权与资金配置增强

### 范围

- Group A 全量
- Group B 中 `interest/query`、`va/transfer`、`va/topup/config`

### 交付标准

- 所有新增写接口强制 `Idempotency-Key`
- 状态/规则变更具备审计字段（operator、reason、updated_at）
- 覆盖单元测试与集成测试（成功路径 + 拒绝路径）

### 状态

**已完成（与主线一致）：** Group A、Group B 中除 `/fund/withdraw` 与 `/fund/transfer` 外的条目已在主线实现。

## M6（中风险）：支付通道与资金流转扩展

### 范围

- Group B 剩余：`withdraw`、`transfer`
- Group C 全量

### 交付标准

- 资金动作必须复用已有 freeze/debit/release 原子语义
- 引入支付试算快照（费率、汇率、有效期）
- 回滚策略：新接口可通过 feature flag 快速下线

### 状态

**已实现：** 对应路由与持久化迁移 `012_add_m6_fund_and_preview.sql`。默认关闭：设置环境变量 `FEATURE_M6_FUNDS=true`（或 `1`）后启用；关闭时路由返回 `404`（`PAY-011`）。

## M7（中高风险）：卡支付与风控合规最小落地

### 范围

- Group D 全量（先规则风控，再卡支付）

### 交付标准

- 风控接口先做规则引擎（黑白名单 + 阈值）版本
- 卡支付先做沙箱通道，禁止直接接生产清算
- 新增告警：风控拦截率、卡通道失败率、补偿堆积

### 状态

**已实现（沙箱）：** `FEATURE_M7_CARD_RISK` 控制；迁移 `013_add_m7_card_kyc_audit.sql`。`/risk/kyc/verify` 标识的是 **Agent DID 背后的资金当事方**，非 AI 运行时；生产需替换为合规身份渠道。

## M8（高风险）：自托管签名链路

### 范围

- Group E 全量

### 交付标准

- 所有会话授权可撤销、可过期、可追踪
- 签名请求全链路可审计（requestId + signatureRequestId）
- 先灰度企业账号，默认不开给全部租户

### 状态

**已实现：** `FEATURE_M8_SELF_HOSTED` 控制；迁移 `014_add_m8_self_host.sql`。`/payment/sign/request` 返回 `signId`（即 signatureRequestId）；HTTP 层记录 `requestId`；持久化模式下会话与签名请求落库。

## 4. 每个里程碑的统一工程约束

- OpenAPI 先行：新增接口必须先更新 `openapi.yaml` 再实现代码。
- 测试先行：新增接口必须包含最少 1 条失败用例（鉴权/参数/限额）。
- 可观测先行：新增接口日志统一带 `requestId` 与业务主键。
- 文档同步：`backend/README.md`、`RELEASE_FINAL_CHECKLIST.md` 同步更新。

## 5. 执行建议

- 每个里程碑拆分为 2~4 个小 PR（Schema/Service/API/Frontend 分层）。
- 优先在持久化模式完成功能，再回填内存模式兼容。
- 对高风险能力（卡支付、自托管）强制开 feature flag。
