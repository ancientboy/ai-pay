## Cursor Cloud specific instructions

- 启动与验证优先参考仓库根 `README.md` 与 `frontend/e2e/README.md`：端到端链路至少需要 `frontend:3000` 与 `backend:8080` 同时运行；MySQL/Redis 仅在需要持久化模式时再启用。
- Bridge 相关（KYC/VA）默认走真实 API：当设置 `BRIDGE_API_KEY` 时，`/billing/provider/bridge/va-countries`、`/billing/provider/bridge/kyc-link`、`/billing/provider/bridge/virtual-account` 会调用 Bridge；未配置时仅国家能力接口回退为 mock，VA 创建会明确报错缺少密钥。
- Billing 页面里的 Bridge 能力卡片已覆盖“查询 VA 支持国家 + 发起 KYC Link + 创建 VA”最小闭环，可直接作为联调 hello-world 入口。
