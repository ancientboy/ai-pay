# AI Pay Frontend Console

Next.js + TypeScript + Tailwind front-end console for the AI payment MVP.

## Setup

```bash
cd frontend
cp .env.example .env.local
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

## Contract & E2E

- OpenAPI contract (backend): `../openapi.yaml`
- Frontend smoke E2E (Playwright):

```bash
cd frontend
npx playwright install chromium
npm run e2e
```

The E2E test expects:
- frontend at `http://127.0.0.1:3000`
- backend at `http://127.0.0.1:8080`

## Environment

- `NEXT_PUBLIC_API_BASE_URL`: backend API endpoint, default `http://127.0.0.1:8080`

## Implemented Pages

- `/dashboard`
- `/agents`
- `/authorize`
- `/recharge`
- `/transactions`
- `/developer`
- `/settings`
- `/login`

## Backend Integration

- Browser requests are proxied through `app/api/backend/[...path]/route.ts`
- This avoids CORS issues for local development
- Main interactive flows now connected:
  - Create Agent + Create Account
  - Browser-side DID key generation + pay request signing (Ed25519)
  - Set Authorize Rule
  - Recharge
  - Pay / Query Status / Query Balance / Query Ledger
- Dashboard reads backend `GET /metrics/overview` for live metrics
- Global toast notifications enabled for success/error feedback
- Agent list and recharge list are loaded from backend query interfaces
- Dashboard includes trend chart using ECharts
- Transaction page supports status filter and pagination
- Core forms validated with zod before API request
- Settings supports runtime API endpoint override via localStorage
- Added route guard middleware and login flow for console pages
- Added detail modal interactions for agent/recharge/transaction rows
