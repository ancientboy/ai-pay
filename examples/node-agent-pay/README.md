# Node.js Agent 支付示例

演示如何用 Node 18+ 调用与 `backend/internal/http/integration_flow_test.go` 相同的流程：

1. `POST /agent/did/register`
2. `POST /account/create`
3. `POST /fund/recharge`
4. `POST /authorize/payment/set`
5. `POST /payment/x402/pay`（Ed25519 签名）
6. `GET /payment/status/query`

## 运行

```bash
cd examples/node-agent-pay
node agent-pay-example.mjs
```

可选环境变量：

- `AI_PAY_BASE_URL`：后端地址，默认 `http://127.0.0.1:8080`

确保本地后端已启动且默认端口可访问。
