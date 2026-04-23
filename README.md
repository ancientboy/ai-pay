# AI Pay MVP

AI Pay 的 MVP 仓库，包含 Go 后端与 Next.js 前端控制台。

## 仓库结构

- `backend/`: 支付核心服务（Go）
- `frontend/`: 控制台（Next.js + TypeScript）
- `scripts/`: 一键启动/状态/停止脚本
- `GUSD-开放AI支付系统-全整合版.md`: 产品与商业化总文档
- `RELEASE_FINAL_CHECKLIST.md`: 上线前最终检查单（执行记录）

## 快速启动（推荐）

### 方案 A：一键联调（MySQL + Redis + 前后端）

```bash
cd /workspace
bash scripts/start_all.sh
bash scripts/status_all.sh
```

默认端口：

- 前端：`3000`
- 后端（持久化模式）：`18080`
- MySQL：`3307`
- Redis：`6379`

停止：

```bash
cd /workspace
bash scripts/stop_all.sh
```

### 方案 B：轻量开发（后端内存模式）

```bash
cd /workspace/backend
cp .env.example .env
go run ./cmd/server
```

后端默认端口 `8080`。若前端沿用 `frontend/.env.example`（`18080`），请在 `frontend/.env.local` 改为：

```bash
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
```

## 前端启动

```bash
cd /workspace/frontend
cp .env.example .env.local
npm install
npm run dev
```

## 质量检查

```bash
cd /workspace/backend
go test ./...

cd /workspace/frontend
npm run lint
```

端到端冒烟（通过前端代理打通全链路）：

```bash
cd /workspace
bash backend/scripts/smoke_test.sh
```

## 发布与回滚

- 灰度/回滚手册：`backend/RELEASE_RUNBOOK.md`
- 最终检查单：`RELEASE_FINAL_CHECKLIST.md`
- 回滚脚本：`backend/scripts/rollback.sh`

