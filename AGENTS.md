## Cursor Cloud specific instructions

- 启动与验证优先参考仓库根 `README.md` 与 `frontend/e2e/README.md`：端到端链路至少需要 `frontend:3000` 与 `backend:8080` 同时运行；MySQL/Redis 仅在需要持久化模式时再启用。
- Bridge 相关（KYC/VA）默认走真实 API：当设置 `BRIDGE_API_KEY` 时，`/billing/provider/bridge/va-countries`、`/billing/provider/bridge/kyc-link`、`/billing/provider/bridge/virtual-account` 会调用 Bridge；未配置时仅国家能力接口回退为 mock，VA 创建会明确报错缺少密钥。
- Billing 页面里的 Bridge 能力卡片已覆盖“查询 VA 支持国家 + 发起 KYC Link + 创建 VA”最小闭环，可直接作为联调 hello-world 入口。
- 角色约定：`admin` 可见「订阅收款」全流程（通道能力、Stripe 结账、Bridge KYC/VA、对账 CSV 导出）；`operator` 使用「账单与对账」视图查看订阅/对账并可操作充值等业务菜单（Agent / 授权规则等），不创建组织级结账；`readonly` 保持只读。代理层对非管理员拦截 `POST /billing/checkout/create`、所有 `/billing/provider/bridge/*`，CSV 导出按钮亦仅管理员可用。
- 控制台首次登录会弹出「新手引导」浮层（`localStorage` 键 `ai-pay.onboardingTour.v1`，值为 `dismissed` 则不再显示）；侧栏有「API 集成」页仅作文档索引（OpenAPI、`docs/API_INTEGRATION.md`、`examples/node-agent-pay`），不改变后端行为。
