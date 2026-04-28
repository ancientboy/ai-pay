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

export function updateAuthorizeRule(input: {
  agentDid: string;
  singleLimit: string;
  dailyLimit: string;
  whitelist: string[];
}) {
  return request("/authorize/payment/update", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function freezeAuthorizeRule(agentDid: string) {
  return request("/authorize/freeze", {
    method: "POST",
    body: JSON.stringify({ agentDid }),
  });
}

export function activateAuthorizeRule(agentDid: string) {
  return request("/authorize/activate", {
    method: "POST",
    body: JSON.stringify({ agentDid }),
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

export function queryInterest(accountId: string) {
  return request<{
    accountId: string;
    annualRate: number;
    accruedInterest: number;
    asOf: string;
  }>(`/account/interest/query?accountId=${encodeURIComponent(accountId)}`);
}

export function getVATopupConfig(accountId: string) {
  return request<{
    accountId: string;
    autoTopupEnabled: boolean;
    thresholdAmount: number;
    targetAmount: number;
    updatedAt: string;
  }>(`/account/va/topup/config?accountId=${encodeURIComponent(accountId)}`);
}

export function setVATopupConfig(input: {
  accountId: string;
  autoTopupEnabled: boolean;
  thresholdAmount: string;
  targetAmount: string;
}) {
  return request<{
    accountId: string;
    autoTopupEnabled: boolean;
    thresholdAmount: number;
    targetAmount: number;
    updatedAt: string;
  }>("/account/va/topup/config", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function transferVA(input: {
  fromAccountId: string;
  toAccountId: string;
  amount: string;
}) {
  return request("/account/va/transfer", {
    method: "POST",
    body: JSON.stringify(input),
    idempotencyKey: `va-transfer-ui-${Date.now()}`,
  });
}

export type VATransferRecord = {
  transferId: string;
  fromAccountId: string;
  toAccountId: string;
  amount: number;
  status: string;
  idempotencyKey: string;
  createdAt: string;
};

export function listVATransfers(input?: {
  accountId?: string;
  status?: string;
  startTime?: string;
  endTime?: string;
  limit?: number;
  offset?: number;
}) {
  const query = new URLSearchParams();
  if (input?.accountId?.trim()) {
    query.set("accountId", input.accountId.trim());
  }
  if (input?.status?.trim()) {
    query.set("status", input.status.trim());
  }
  if (input?.startTime?.trim()) {
    query.set("startTime", input.startTime.trim());
  }
  if (input?.endTime?.trim()) {
    query.set("endTime", input.endTime.trim());
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
  return request<VATransferRecord[]>(`/account/va/transfer/list${suffix ? `?${suffix}` : ""}`);
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

export type RiskConfig = {
  enabled: boolean;
  singleAmountLimit: number;
  blockedMerchants: string[];
  updatedAt: string;
};

export type ChannelRoute = {
  merchantId: string;
  mode: "SETTLE" | "ASYNC" | "FAIL";
  updatedAt: string;
};

export type AuditLog = {
  id: number;
  actor: string;
  role: string;
  action: string;
  resource: string;
  requestId: string;
  detail: Record<string, unknown>;
  createdAt: string;
};

export type AuthSession = {
  sessionId: string;
  agentDid: string;
  status: string;
  expiresAt: string;
  createdAt: string;
};

export type PaymentSignRequestRecord = {
  signId: string;
  agentDid: string;
  merchantId: string;
  amount: string;
  sessionId: string;
  status: string;
  expiresAt: string;
  createdAt: string;
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

export function getRiskConfig() {
  return request<RiskConfig>("/developer/risk-config");
}

export function setRiskConfig(input: {
  enabled: boolean;
  singleAmountLimit: string;
  blockedMerchants: string[];
}) {
  return request<RiskConfig>("/developer/risk-config", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function listChannelRoutes() {
  return request<ChannelRoute[]>("/developer/channel-routes");
}

export function setChannelRoute(input: { merchantId: string; mode: "SETTLE" | "ASYNC" | "FAIL" }) {
  return request<ChannelRoute>("/developer/channel-routes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function deleteChannelRoute(merchantId: string) {
  return request(`/developer/channel-routes?merchantId=${encodeURIComponent(merchantId)}`, {
    method: "DELETE",
  });
}

export function listAuditLogs(input?: { action?: string; resource?: string; limit?: number; offset?: number }) {
  const query = new URLSearchParams();
  if (input?.action?.trim()) {
    query.set("action", input.action.trim());
  }
  if (input?.resource?.trim()) {
    query.set("resource", input.resource.trim());
  }
  if (input?.limit && Number.isFinite(input.limit) && input.limit > 0) {
    query.set("limit", String(input.limit));
  }
  if (typeof input?.offset === "number" && Number.isFinite(input.offset) && input.offset >= 0) {
    query.set("offset", String(input.offset));
  }
  const suffix = query.toString();
  return request<AuditLog[]>(`/developer/audit-logs${suffix ? `?${suffix}` : ""}`);
}

function buildWalletBindSignPayload(
  agentDid: string,
  walletAddress: string,
  idempotencyKey: string,
  signTimestamp: string,
) {
  return `wallet_bind|${agentDid}|${walletAddress}|${idempotencyKey}|${signTimestamp}`;
}

function buildWalletUnbindSignPayload(
  agentDid: string,
  idempotencyKey: string,
  signTimestamp: string,
) {
  return `wallet_unbind|${agentDid}|${idempotencyKey}|${signTimestamp}`;
}

function buildSessionCreateSignPayload(
  agentDid: string,
  idempotencyKey: string,
  signTimestamp: string,
) {
  return `session_create|${agentDid}|${idempotencyKey}|${signTimestamp}`;
}

function buildSessionRevokeSignPayload(
  agentDid: string,
  sessionId: string,
  idempotencyKey: string,
  signTimestamp: string,
) {
  return `session_revoke|${agentDid}|${sessionId}|${idempotencyKey}|${signTimestamp}`;
}

function buildSignRequestPayload(
  agentDid: string,
  merchantId: string,
  amount: string,
  sessionId: string,
  idempotencyKey: string,
  signTimestamp: string,
) {
  return `sign_request|${agentDid}|${merchantId}|${amount}|${sessionId}|${idempotencyKey}|${signTimestamp}`;
}

export function bindWallet(input: { agentDid: string; walletAddress: string; label?: string }) {
  const idempotencyKey = `wallet-bind-ui-${Date.now()}`;
  const signTimestamp = new Date().toISOString();
  return (async () => {
    let signature = "sig";
    try {
      signature = await signAgentPayload(
        input.agentDid,
        buildWalletBindSignPayload(input.agentDid, input.walletAddress, idempotencyKey, signTimestamp),
      );
    } catch {
      // Compatibility fallback for legacy agents without DID key pair.
    }
    return request("/wallet/bind", {
      method: "POST",
      body: JSON.stringify({
        agentDid: input.agentDid,
        walletAddress: input.walletAddress,
        label: input.label ?? "",
        signature,
      }),
      idempotencyKey,
      signTimestamp,
    });
  })();
}

export function walletUnbind(agentDid: string) {
  const idempotencyKey = `wallet-unbind-ui-${Date.now()}`;
  const signTimestamp = new Date().toISOString();
  return (async () => {
    let signature = "sig";
    try {
      signature = await signAgentPayload(
        agentDid,
        buildWalletUnbindSignPayload(agentDid, idempotencyKey, signTimestamp),
      );
    } catch {
      // Compatibility fallback for legacy agents without DID key pair.
    }
    return request("/wallet/unbind", {
      method: "POST",
      body: JSON.stringify({ agentDid, signature }),
      idempotencyKey,
      signTimestamp,
    });
  })();
}

export function createAuthSession(input: { agentDid: string; ttlMinutes?: number }) {
  const idempotencyKey = `session-create-ui-${Date.now()}`;
  const signTimestamp = new Date().toISOString();
  return (async () => {
    let signature = "sig";
    try {
      signature = await signAgentPayload(
        input.agentDid,
        buildSessionCreateSignPayload(input.agentDid, idempotencyKey, signTimestamp),
      );
    } catch {
      // Compatibility fallback for legacy agents without DID key pair.
    }
    return request<AuthSession>("/authorize/session/create", {
      method: "POST",
      body: JSON.stringify({
        agentDid: input.agentDid,
        ttlMinutes: input.ttlMinutes,
        signature,
      }),
      idempotencyKey,
      signTimestamp,
    });
  })();
}

export function revokeAuthSession(input: { agentDid: string; sessionId: string }) {
  const idempotencyKey = `session-revoke-ui-${Date.now()}`;
  const signTimestamp = new Date().toISOString();
  return (async () => {
    let signature = "sig";
    try {
      signature = await signAgentPayload(
        input.agentDid,
        buildSessionRevokeSignPayload(input.agentDid, input.sessionId, idempotencyKey, signTimestamp),
      );
    } catch {
      // Compatibility fallback for legacy agents without DID key pair.
    }
    return request("/authorize/session/revoke", {
      method: "POST",
      body: JSON.stringify({
        agentDid: input.agentDid,
        sessionId: input.sessionId,
        signature,
      }),
      idempotencyKey,
      signTimestamp,
    });
  })();
}

export function createPaymentSignRequest(input: {
  agentDid: string;
  merchantId: string;
  amount: string;
  sessionId?: string;
}) {
  const idempotencyKey = `sign-request-ui-${Date.now()}`;
  const signTimestamp = new Date().toISOString();
  const sessionId = input.sessionId ?? "";
  return (async () => {
    let signature = "sig";
    try {
      signature = await signAgentPayload(
        input.agentDid,
        buildSignRequestPayload(
          input.agentDid,
          input.merchantId,
          input.amount,
          sessionId,
          idempotencyKey,
          signTimestamp,
        ),
      );
    } catch {
      // Compatibility fallback for legacy agents without DID key pair.
    }
    return request<PaymentSignRequestRecord>("/payment/sign/request", {
      method: "POST",
      body: JSON.stringify({
        agentDid: input.agentDid,
        merchantId: input.merchantId,
        amount: input.amount,
        sessionId,
        signature,
      }),
      idempotencyKey,
      signTimestamp,
    });
  })();
}

// Backward-compatible alias for earlier page import.
export const requestPaymentSign = createPaymentSignRequest;

export function submitPaymentSign(input: {
  signId: string;
  payerDid: string;
  merchantId: string;
  amount: string;
}) {
  const idempotencyKey = `sign-submit-ui-${Date.now()}`;
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
    return request<{ transactionId: string; status: string }>("/payment/sign/submit", {
      method: "POST",
      body: JSON.stringify({
        signId: input.signId,
        payerDid: input.payerDid,
        merchantId: input.merchantId,
        amount: input.amount,
        idempotencyKey,
        signature,
      }),
      signTimestamp,
    });
  })();
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
