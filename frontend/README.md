# AI Pay Frontend Console

Next.js + TypeScript + Tailwind front-end console for the AI payment MVP.

## Setup

```bash
cd frontend
cp .env.example .env.local
npm install
npm run dev
```

Open [http://127.0.0.1:3000](http://127.0.0.1:3000).

## Environment

- `NEXT_PUBLIC_API_BASE_URL`: backend API endpoint.
- Default value in `.env.example`: `http://127.0.0.1:8080`
- Runtime override supported in Settings page (stored in localStorage).

## Dev origin note (Next.js 16)

This repository allows both `localhost` and `127.0.0.1` as development origins
to avoid blocked dev resources when opening the app from `http://127.0.0.1:3000`.

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

- Browser requests are proxied through `app/api/backend/[...path]/route.ts`.
- This avoids CORS issues for local development.
- Main interactive flows now connected:
  - Create Agent + Create Account
  - Browser-side DID key generation + pay request signing (Ed25519)
  - Set Authorize Rule
  - Recharge
  - Pay / Query Status / Query Balance / Query Ledger
- Dashboard reads backend `GET /metrics/overview` for live metrics.
- Global toast notifications enabled for success/error feedback.
- Agent list and recharge list are loaded from backend query interfaces.
- Dashboard includes trend chart using ECharts.
- Transaction page supports status filter and pagination.
- Core forms validated with zod before API request.
- Added route guard middleware and login flow for console pages.
- Added detail modal interactions for agent/recharge/transaction rows.
