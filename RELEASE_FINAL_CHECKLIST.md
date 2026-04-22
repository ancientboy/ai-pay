# AI Pay MVP 上线前最终检查单（最终版）

## 一、环境与依赖

- [ ] `docker compose` 中 MySQL/Redis 正常运行
- [ ] 后端端口 `18080` 健康检查通过：`/health`、`/ready`
- [ ] 前端端口 `3000` 可访问且代理后端正常
- [ ] 执行数据库迁移（按顺序）：
  - `001_init.sql`
  - `002_add_fee_and_recharge_log.sql`
  - `003_add_va_card_no.sql`
  - `004_add_account_hold.sql`
  - `005_add_agent_did_pub_key.sql`
  - `006_add_pay_order_hold_id.sql`

## 二、核心能力验收

- [ ] Agent 开户链路：`/agent/did/register` + `/account/create`
- [ ] DID 公钥签名校验：无公钥/签名错误请求被拦截（`PAY-001`）
- [ ] 充值幂等：`/fund/recharge` 强制 `Idempotency-Key`
- [ ] 支付幂等：`/payment/x402/pay` 同 key 不重复扣款
- [ ] 账务原子性：冻结/扣减/释放（`freeze/debit/release`）
- [ ] 状态机能力：
  - `SETTLING` 创建
  - `/payment/status/callback` 可转 `SETTLED/FAILED`
  - 超时补偿可回滚冻结资金
- [ ] 资金动作接口：
  - `/payment/unfreeze`
  - `/payment/refund`

## 三、安全与观测

- [ ] 签名时间窗校验生效（`X-Sign-Timestamp`）
- [ ] 支付限流生效（IP + Agent DID）
- [ ] 请求链路含 `X-Request-Id`
- [ ] 敏感日志脱敏（DID/签名不明文）
- [ ] 对账任务与补偿任务运行正常

## 四、测试与联调

- [ ] 后端测试通过：`cd backend && go test ./...`
- [ ] 前端检查通过：`cd frontend && npm run lint && npm run build`
- [ ] 端到端冒烟通过：`bash ./backend/scripts/smoke_test.sh`

## 五、发布与回滚

- [ ] 发布前记录版本号、迁移版本、配置快照
- [ ] 灰度放量：10% -> 30% -> 100%
- [ ] 监控阈值（失败率、超时率、补偿堆积）已设定
- [ ] 回滚预案可执行：
  - 参考：`backend/RELEASE_RUNBOOK.md`
  - 脚本：`backend/scripts/rollback.sh`

## 六、建议上线说明模板

上线内容：

- DID 公钥签名强校验（支付请求）
- 充值/支付幂等增强
- 支付状态机扩展（SETTLING + callback + compensation）
- 资金动作 API（unfreeze/refund）
- Agent VA 卡号能力与前端充值入口优化

风险提示：

- 新签名策略会拒绝未注册公钥的 Agent
- 回调链路新增后，需关注 `SETTLING` 堆积与补偿告警

回滚条件：

- 支付失败率异常升高
- `SETTLING` 长时间堆积无法消化
- 对账异常持续出现
