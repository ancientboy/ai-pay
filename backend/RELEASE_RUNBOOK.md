# M4 灰度上线与回滚预案

## 灰度上线步骤

1. **预发布验证**
   - 执行：`go test ./...`
   - 检查关键接口：开户、充值、授权、支付、状态查询
   - 生产环境确认安全变量已配置：`ADMIN_BEARER_TOKEN`、`CALLBACK_TOKEN`
   - 如需验证出站 webhook 验签，配置：`WEBHOOK_SIGNING_SECRET`
   - 与回调方联调并确认请求头：
     - `X-Callback-Token`
     - `X-Callback-Timestamp`
     - `X-Callback-Nonce`
     - `X-Callback-Idempotency-Key`
     - `X-Callback-Signature-Version`、`X-Callback-Signature`（启用 `CALLBACK_SIGNING_SECRET` 时必填）
2. **数据库迁移**
   - 顺序执行：`001_init.sql` -> `002_add_fee_and_recharge_log.sql` -> `003_add_va_card_no.sql` -> `004_add_account_hold.sql` -> `005_add_agent_did_pub_key.sql` -> `006_add_pay_order_hold_id.sql` -> `007_add_developer_resources.sql` -> `008_add_webhook_delivery_task.sql` -> `009_add_va_topup_and_transfer.sql` -> `010_add_audit_log.sql` -> `011_add_risk_and_channel_route.sql` -> `012_add_m6_fund_and_preview.sql` -> `013_add_m7_card_kyc_audit.sql`
   - 校验表结构与索引是否创建成功
3. **小流量灰度**
   - 先仅开放 10% Agent DID 到新版本
   - 观察 30 分钟：支付成功率、429 比例、`[ALERT]` 告警数
4. **扩大流量**
   - 10% -> 30% -> 50% -> 100%
   - 每个阶段至少观察 15 分钟

## 关键监控指标

- 支付成功率（目标 >= 95%）
- 支付接口 P95 延迟（目标 <= 300ms）
- 余额不一致告警（目标 = 0）
- `PAY-003` 占比（余额不足，确认是否异常突增）
- 429 限流占比（确认是否限流配置过严）
- `GET /ready` 结果（持久化模式需稳定返回 `ready=true`）
- Webhook 投递成功率与死信新增量（死信应保持低位，异常增长需立即排查）
- 死信重放可用性（`POST /developer/webhook-deliveries/replay`）
- VA 转账列表筛选正确性（`GET /account/va/transfer/list` 的 `status/startTime/endTime`）
- 资金页分页与导出一致性（同一筛选条件下，列表结果与 CSV 一致）

## 回滚触发条件

- 支付成功率连续 5 分钟 < 90%
- 对账不一致告警连续出现
- 补偿任务出现大量 `SETTLING -> FAILED`
- 核心接口 5xx 明显升高

## 回滚操作

- 执行：`bash scripts/rollback.sh`
- 回滚后验证：
  - 健康检查通过：`/health` 返回 `status=ok`，`/ready` 返回 `ready=true`
  - 端到端冒烟通过：`bash scripts/smoke_test.sh`
  - 无新增对账告警
  - 支付成功率恢复
