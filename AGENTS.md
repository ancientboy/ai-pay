## Cursor Cloud specific instructions

- 启动与验证优先参考仓库根 `README.md` 与 `frontend/e2e/README.md`：端到端链路至少需要 `frontend:3000` 与 `backend:8080` 同时运行；MySQL/Redis 仅在需要持久化模式时再启用。
- Bridge 相关（KYC/VA）默认走真实 API：当设置 `BRIDGE_API_KEY` 时，`/billing/provider/bridge/va-countries`、`/billing/provider/bridge/kyc-link`、`/billing/provider/bridge/virtual-account` 会调用 Bridge；未配置时仅国家能力接口回退为 mock，VA 创建会明确报错缺少密钥。
- Billing 页面里的 Bridge 能力卡片已覆盖“查询 VA 支持国家 + 发起 KYC Link + 创建 VA”最小闭环，可直接作为联调 hello-world 入口。
- 角色约定：`admin` 可见「订阅收款」全流程（通道能力、Stripe 结账、Bridge KYC/VA、对账 CSV 导出）；`operator` 使用「账单与对账」视图查看订阅/对账并可操作充值等业务菜单（Agent / 授权规则等），不创建组织级结账；`readonly` 保持只读。代理层对非管理员拦截 `POST /billing/checkout/create`、所有 `/billing/provider/bridge/*`，CSV 导出按钮亦仅管理员可用。
- 套餐：`free`（注册默认）不包含平台结账、也不含 `billing.bridge_onboarding`；**Starter 及以上**才具备在控制台发起 Bridge KYC/VA 的**资格**（仍须 Bridge 侧 KYC 与 `BRIDGE_API_KEY`）。能力键见 `frontend/lib/plan-capabilities.ts`。**管理员**发起结账与 Bridge 代理请求同样受套餐约束；`/billing/provider/bridge/*` 在非付费档返回 `AUTH-014`。`free` 下第二个 `POST /agent/did/register` 返回 `AUTH-013`（全局列表计数；生产按租户计数）。
- 对外品牌为 **AgentTrust Pay（可信付）**；侧栏与公开页顶栏可显示英文主名 + 中文副名。
- `merchantId` 为租户在「授权规则」里配置的收款方标识。要接**链上稳定币 / 外部 x402**：在 `channel_route` 把该 `merchantId` 配为 `ASYNC`，并设置 `EXTERNAL_SETTLEMENT_WEBHOOK_URL` 指向自研中继；订单 `SETTLING` 时后端会 POST `payment.settling` 到该 URL，中继完成实际结算后调 `POST /payment/status/callback`（`CALLBACK_TOKEN` 等）销账。未配 Webhook 时异步单仍会在库中保持 SETTLING，需人工或另行对接。
- 控制台首次登录会弹出「新手引导」浮层（`localStorage` 键 `ai-pay.onboardingTour.v1`，值为 `dismissed` 则不再显示）；侧栏「API 集成」与**公开页** `/docs/integration`（无需登录，内容与控制台一致）均提供小白向完整流程（注册/开户 → VA 充值 → 授权 → 调支付 API → 查交易），并附 OpenAPI / `docs/API_INTEGRATION.md` / Node 示例等仓库路径说明，不改变后端行为。
