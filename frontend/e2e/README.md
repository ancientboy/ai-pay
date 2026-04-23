# Frontend E2E Smoke Test

This directory contains Playwright-based frontend smoke tests.

## Run locally

```bash
cd frontend
npx playwright install chromium
npm run e2e
```

## Preconditions

- frontend running at `http://127.0.0.1:3000`
- backend running at `http://127.0.0.1:8080`
