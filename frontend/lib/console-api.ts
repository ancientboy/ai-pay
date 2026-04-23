import { ApiClientError } from "@/lib/error-map";
import { buildPaySignPayload, signAgentPayload } from "@/lib/agent-signature";

type ApiResponse<T> = {
  code: string;
  message?: string;
  data: T;
};

const STORAGE_API_BASE_URL_KEY = "ai-pay.apiBaseUrl";

function getRuntimeBaseURL() {
  if (typeof window === "undefined") {
    return "";
  }
  const value = window.localStorage.getItem(STORAGE_API_BASE_URL_KEY);
  return value?.trim() ?? "";
}

async function request<T>(
  path: string,
  init?: RequestInit & { idempotencyKey?: string; signTimestamp?: string },
) {
  const headers = new Headers(init?.headers);
  if (init?.idempotencyKey) {
    headers.set("Idempotency-Key", init.idempotencyKey);
  }
  if (init?.signTimestamp) {
    headers.set("X-Sign-Timestamp", init.signTimestamp);
  }
  if (init?.body && !headers.get("content-type")) {
    headers.set("Content-Type", "application/json");
  }
  const runtimeBaseURL = getRuntimeBaseURL();
  if (runtimeBaseURL) {
    headers.set("X-Api-Base-Url", runtimeBaseURL);
  }

  const response = await fetch(`/api/backend${path}`, {
    ...init,
    headers,
  });
  let payload: ApiResponse<T> | null = null;
  try {
    payload = (await response.json()) as ApiResponse<T>;
  } catch {
    payload = null;
  }
  if (!response.ok || payload?.code !== "0") {
    const requestId =
      response.headers.get("x-request-id") ||
      (payload?.data &&
      typeof payload.data === "object" &&
      payload.data !== null &&
      "requestId" in payload.data
        ? String((payload.data as { requestId?: string }).requestId ?? "")
        : undefined);
    throw new ApiClientError(
      payload?.code || "PAY-010",
      payload?.message || "request failed",
      requestId,
    );
  }
  if (!payload) {
    throw new ApiClientError("PAY-010", "request failed");
  }
  return payload.data;
}

export function getSavedApiBaseURL() {
  return getRuntimeBaseURL();
}

export function saveApiBaseURL(url: string) {
  if (typeof window === "undefined") {
    return;
  }
  if (!url.trim()) {
    window.localStorage.removeItem(STORAGE_API_BASE_URL_KEY);
    return;
  }
  window.localStorage.setItem(STORAGE_API_BASE_URL_KEY, url.trim());
}

export function registerAgent(agentDid: string, didPubKey?: string) {
  return request<{ DID: string }>("/agent/did/register", {
    method: "POST",
    body: JSON.stringify({ agentDid, didPubKey }),
  });
}

export function createAccount(agentDid: string) {
  return request<{ WalletAddress: string; VAAccountID: string; VACardNo: string; AgentDID: string }>(
    "/account/create",
    {
      method: "POST",
      body: JSON.stringify({ agentDid }),
    },
  );
}

export function setAuthorizeRule(input: {
  agentDid: string;
  singleLimit: string;
  dailyLimit: string;
  whitelist: string[];
}) {
  return request("/authorize/payment/set", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function recharge(input: { vaAccountId?: string; vaCardNo?: string; amount: string }) {
  return request("/fund/recharge", {
    method: "POST",
    body: JSON.stringify(input),
    idempotencyKey: `rch-ui-${Date.now()}`,
  });
}

export function pay(input: {
  payerDid: string;
  merchantId: string;
  amount: string;
}) {
  const idempotencyKey = `idem-ui-${Date.now()}`;
  const signTimestamp = new Date().toISOString();
  return (async () => {
    let signature = "sig";
    try {
      signature = await signAgentPayload(
        input.payerDid,
        buildPaySignPayload(
          input.payerDid,
          input.merchantId,
          input.amount,
          idempotencyKey,
          signTimestamp,
        ),
      );
    } catch {
      // Compatibility fallback for legacy agents without DID key pair.
    }
  return request<{ transactionId: string; status: string }>("/payment/x402/pay", {
    method: "POST",
    body: JSON.stringify({ ...input, signature }),
    idempotencyKey,
    signTimestamp,
  });
  })();
}

export function queryTransaction(transactionId: string) {
  return request<{
    ID: string;
    PayerDID: string;
    Merchant: string;
    Amount: number;
    Fee?: number;
    Status: string;
    CreatedAt: string;
  }>(`/payment/status/query?transactionId=${encodeURIComponent(transactionId)}`);
}

export function queryBalance(accountId: string) {
  return request<{ balance: number }>(
    `/account/balance/query?accountId=${encodeURIComponent(accountId)}`,
  );
}

export function queryLedger(accountId: string) {
  return request<
    Array<{
      ID: string;
      Merchant: string;
      Amount: number;
      Fee?: number;
      Status: string;
      CreatedAt: string;
    }>
  >(`/account/ledger/query?accountId=${encodeURIComponent(accountId)}`);
}

export function listAgents() {
  return request<
    Array<{
      agentDid: string;
      vaAccountId: string;
      vaCardNo: string;
      walletAddress: string;
      balance: number;
      status: string;
    }>
  >("/agent/list");
}

export function listRecharges(accountId?: string, limit = 20) {
  const query = new URLSearchParams();
  if (accountId) {
    query.set("accountId", accountId);
  }
  query.set("limit", String(limit));
  return request<
    Array<{
      rechargeId: string;
      vaAccountId: string;
      amount: number;
      status: string;
      createdAt: string;
    }>
  >(`/fund/recharge/list?${query.toString()}`);
}

export type DeveloperAPIKey = {
  id: string;
  name: string;
  key: string;
  createdAt: string;
};

export type DeveloperWebhook = {
  id: string;
  url: string;
  event: string;
  createdAt: string;
};

export type WebhookDeliveryStatus = "PENDING" | "RETRYING" | "SENT" | "DEAD";

export type DeveloperWebhookDelivery = {
  id: number;
  webhookId: string;
  url: string;
  event: string;
  dedupeKey: string;
  payload: Record<string, unknown>;
  status: WebhookDeliveryStatus;
  attempts: number;
  maxAttempts: number;
  nextRetryAt: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
};

export type DeveloperWebhookDeliveryStats = {
  pending: number;
  retrying: number;
  sent: number;
  dead: number;
  total: number;
};

export function listApiKeys() {
  return request<DeveloperAPIKey[]>("/developer/api-keys");
}

export function createApiKey(name: string) {
  return request<DeveloperAPIKey>("/developer/api-keys", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export function listWebhooks() {
  return request<DeveloperWebhook[]>("/developer/webhooks");
}

export function createWebhook(url: string, event: string) {
  return request<DeveloperWebhook>("/developer/webhooks", {
    method: "POST",
    body: JSON.stringify({ url, event }),
  });
}

export function listWebhookDeliveries(input?: {
  status?: WebhookDeliveryStatus | "";
  event?: string;
  webhookId?: string;
  limit?: number;
  offset?: number;
}) {
  const query = new URLSearchParams();
  if (input?.status) {
    query.set("status", input.status);
  }
  if (input?.event && input.event.trim()) {
    query.set("event", input.event.trim());
  }
  if (input?.webhookId && input.webhookId.trim()) {
    query.set("webhookId", input.webhookId.trim());
  }
  if (input?.limit && Number.isFinite(input.limit) && input.limit > 0) {
    query.set("limit", String(input.limit));
  }
  if (
    typeof input?.offset === "number" &&
    Number.isFinite(input.offset) &&
    input.offset >= 0
  ) {
    query.set("offset", String(input.offset));
  }
  const suffix = query.toString();
  return request<DeveloperWebhookDelivery[]>(
    `/developer/webhook-deliveries${suffix ? `?${suffix}` : ""}`,
  );
}

export function getWebhookDeliveryStats() {
  return request<DeveloperWebhookDeliveryStats>("/developer/webhook-deliveries/stats");
}

export function replayWebhookDelivery(id: number) {
  return request("/developer/webhook-deliveries/replay", {
    method: "POST",
    body: JSON.stringify({ id }),
  });
}

// Backward-compatible aliases for pages using older names.
export const listDeveloperApiKeys = listApiKeys;
export const createDeveloperApiKey = createApiKey;
export const listDeveloperWebhooks = listWebhooks;
export function createDeveloperWebhook(input: { url: string; event: string }) {
  return createWebhook(input.url, input.event);
}
export const listDeveloperWebhookDeliveries = listWebhookDeliveries;
export const getDeveloperWebhookDeliveryStats = getWebhookDeliveryStats;
