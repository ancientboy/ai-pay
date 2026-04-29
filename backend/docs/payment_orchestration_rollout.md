# Payment Orchestration Rollout Plan (M9)

## 目标
- 建立统一“本地控制账本 + 外部 Provider 子账户”支付编排层。
- 将支付执行从“直连单通道”演进为“意图驱动 + 路由执行 + 可追踪执行记录”。

## 已完成（本轮之前 + 本轮）
1. 核心表结构（migration 022）
   - `provider_sub_account`
   - `payment_intent`
   - `payment_execution`
2. 后端骨架 API
   - 绑定子账户：`POST /orchestrate/provider-account/bind`
   - 查询子账户：`GET /orchestrate/provider-account/list`
   - 创建意图：`POST /orchestrate/payment-intent/create`
   - 执行意图：`POST /orchestrate/payment-intent/execute`
   - 查询状态：`GET /orchestrate/payment-intent/status`
3. 语义增强（本轮）
   - 创建意图时默认自动路由（按 `platform_va + currency` 选最新激活子账户）
   - 执行前强校验：仅 `CREATED` 状态可执行
   - 执行前强校验：必须有 `route_provider + route_sub_account`
   - 执行接口支持显式覆盖 provider，并二次解析子账户路由

## 分阶段推进

### Phase A（已落地）
- 数据模型与 API 骨架可运行。
- 支持最小闭环：绑定子账户 -> 创建意图 -> 执行意图 -> 查询状态。

### Phase B（进行中）
- 路由策略增强：
  - 按币种/通道优先级路由
  - 按风控结果动态降级路由
- 执行状态增强：
  - `CREATED -> PROCESSING -> SETTLED/FAILED`
  - 异步回调对账后更新最终态

### Phase C（待开始）
- Provider Adapter 统一执行器
  - `bridge`, `stripe`, `mock` 统一 `execute(intent)` 协议
  - 落库原始 provider 回包用于审计与重放
- Webhook 编排
  - provider 交易结果映射回 intent/execution

### Phase D（待开始）
- 前端控制台编排页
  - Provider 子账户管理
  - 意图创建与执行调试
  - 执行历史与失败原因可视化

## 测试建议（可复用）
1. 自动化：
   - `go test ./...`（后端全量）
2. 端到端 API 冒烟：
   - 注册 agent / 创建账户
   - 绑定 provider 子账户
   - 创建并执行 payment intent
   - 查询 intent + executions 一致性

## 风险与约束
- 当前执行仍为“框架化模拟”，真实扣款由后续 Provider Adapter 接管。
- 路由策略现为简化规则（最新激活记录优先），后续需引入优先级配置与风控权重。
