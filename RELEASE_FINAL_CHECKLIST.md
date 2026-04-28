# AI Pay MVP 上线前最终检查单（执行记录版）

> 说明：本文件用于记录一次具体发布窗口的验收结果。  
> 每次发布建议复制本文件（含版本号/日期）并填写执行人、结果与备注。

## 发布批次信息

- 发布批次：`v2026.04-mvp-hardening`
- 目标分支：`main`
- 执行日期：`2026-04-22`
- 执行人：`cloud-agent`
- 参考文档：
  - `backend/RELEASE_RUNBOOK.md`
  - `backend/scripts/rollback.sh`

## 一、环境与依赖

- [ ] `docker compose` 中 MySQL/Redis 正常运行（需目标环境具备 Docker）
- [x] 后端端口 `8080` 健康检查通过：`/health`、`/ready`（内存模式验证）
- [x] 前端端口 `3000` 可访问且代理后端正常
- [ ] 执行数据库迁移（按顺序）：
  - `001_init.sql`
  - `002_add_fee_and_recharge_log.sql`
  - `003_add_va_card_no.sql`
  - `004_add_account_hold.sql`
  - `005_add_agent_did_pub_key.sql`
  - `006_add_pay_order_hold_id.sql`
   - …（直至 `011_add_risk_and_channel_route.sql`，完整列表见 `backend/RELEASE_RUNBOOK.md`）
   - `012_add_m6_fund_and_preview.sql`（若启用里程碑 M6：`FEATURE_M6_FUNDS=true`）
   - `013_add_m7_card_kyc_audit.sql`（若启用里程碑 M7：`FEATURE_M7_CARD_RISK=true`）
   - `014_add_m8_self_host.sql`（若启用里程碑 M8：`FEATURE_M8_SELF_HOSTED=true`）

## 二、核心能力验收

- [x] Agent 开户链路：`/agent/did/register` + `/account/create`
- [x] DID 公钥签名校验：无公钥/签名错误请求被拦截（`PAY-001`）
- [x] 充值幂等：`/fund/recharge` 强制 `Idempotency-Key`
- [x] 支付幂等：`/payment/x402/pay` 同 key 不重复扣款
- [x] 账务原子性：冻结/扣减/释放（`freeze/debit/release`）
- [x] 状态机能力：
  - `SETTLING` 创建
  - `/payment/status/callback` 可转 `SETTLED/FAILED`
  - 超时补偿可回滚冻结资金
- [x] 资金动作接口：
  - `/payment/unfreeze`
  - `/payment/refund`

## 三、安全与观测

- [x] 签名时间窗校验生效（`X-Sign-Timestamp`）
- [x] 支付限流生效（IP + Agent DID）
- [x] 请求链路含 `X-Request-Id`
- [x] 敏感日志脱敏（DID/签名不明文）
- [ ] 对账任务与补偿任务运行正常（需持久化模式运行后观察）

## 四、测试与联调

- [x] 后端测试通过：`cd backend && go test ./...`
- [x] 前端检查通过：`cd frontend && npm run lint`
- [x] 端到端冒烟通过：`bash ./backend/scripts/smoke_test.sh`

## 五、发布与回滚

- [ ] 发布前记录版本号、迁移版本、配置快照
- [ ] 灰度放量：10% -> 30% -> 100%
- [ ] 监控阈值（失败率、超时率、补偿堆积）已设定
- [x] 回滚预案可执行（文档/脚本已具备）：
  - `backend/RELEASE_RUNBOOK.md`
  - `backend/scripts/rollback.sh`

## 六、本次执行备注

- 当前验证环境无 Docker，已完成内存模式端到端验收；持久化模式项需在具备 Docker 的环境补跑。
- `backend/scripts/smoke_test.sh` 已覆盖登录、开户、充值、授权、支付、状态/余额/流水、限额、概览与列表接口。

## 七、建议上线说明模板

上线内容：

- DID 公钥签名强校验（支付请求）
- 充值/支付幂等增强
- 支付状态机扩展（`SETTLING` + callback + compensation）
- 资金动作 API（`unfreeze` / `refund`）
- Agent VA 卡号能力与前端充值入口优化

风险提示：

- 新签名策略会拒绝未注册公钥的 Agent
- 回调链路新增后，需关注 `SETTLING` 堆积与补偿告警

回滚条件：

- 支付失败率异常升高
- `SETTLING` 长时间堆积无法消化
- 对账异常持续出现
