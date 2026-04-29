const ERROR_MAP: Record<string, string> = {
  "PAY-001": "签名校验失败或签名时间戳无效",
  "PAY-002": "授权规则不通过（限额或白名单）",
  "PAY-003": "账户余额不足",
  "PAY-004": "自动补足失败",
  "PAY-005": "虚拟卡状态不可用",
  "PAY-006": "交易被风控拦截",
  "PAY-007": "通道超时，请稍后重试",
  "PAY-008": "幂等键冲突或缺失",
  "PAY-009": "清算失败，已回滚",
  "PAY-010": "系统繁忙或请求参数无效",
  "AUTH-001": "用户名和密码不能为空",
  "AUTH-002": "用户名或密码错误",
  "AUTH-003": "登录尝试过于频繁，请稍后重试",
};

const ERROR_MAP_EN: Record<string, string> = {
  "PAY-001": "Signature validation failed or timestamp is invalid",
  "PAY-002": "Authorization rule rejected (limit/whitelist)",
  "PAY-003": "Insufficient balance",
  "PAY-004": "Auto top-up failed",
  "PAY-005": "Virtual card is unavailable",
  "PAY-006": "Blocked by risk control",
  "PAY-007": "Channel timeout, please retry",
  "PAY-008": "Idempotency key conflict or missing",
  "PAY-009": "Settlement failed and rolled back",
  "PAY-010": "System busy or invalid request",
  "AUTH-001": "Username and password are required",
  "AUTH-002": "Invalid username or password",
  "AUTH-003": "Too many login attempts, please retry later",
};

export class ApiClientError extends Error {
  code: string;
  requestId?: string;
  rawMessage?: string;

  constructor(code: string, message: string, requestId?: string) {
    super(message);
    this.code = code;
    this.requestId = requestId;
    this.rawMessage = message;
    this.name = "ApiClientError";
  }
}

export function toReadableError(err: unknown, locale: "zh-CN" | "en-US" = "zh-CN"): string {
  const map = locale === "en-US" ? ERROR_MAP_EN : ERROR_MAP;
  const unknown = locale === "en-US" ? "Unknown error" : "未知错误";
  if (err instanceof ApiClientError) {
    const friendly = map[err.code];
    const wrappedCode = locale === "en-US" ? `(${err.code})` : `（${err.code}）`;
    if (friendly) {
      return `${friendly} ${wrappedCode}`;
    }
    return `${err.message} ${wrappedCode}`;
  }
  if (err instanceof Error) {
    return err.message || unknown;
  }
  return unknown;
}

export function errorActionHint(code?: string, locale: "zh-CN" | "en-US" = "zh-CN"): string {
  const c = (code ?? "").trim().toUpperCase();
  const zh: Record<string, string> = {
    "PAY-001": "请检查 DID 公钥、签名内容与签名时间戳（5分钟内）。",
    "PAY-002": "请先在授权规则页检查单笔/日限额与商户白名单。",
    "PAY-003": "请先充值或减少支付金额后重试。",
    "PAY-006": "请检查风控配置（禁用商户、单笔限额）并适当放行。",
    "PAY-007": "通道超时，建议保留 requestId 并稍后重试。",
    "PAY-008": "请确保请求带有唯一且未复用的 Idempotency-Key。",
    "PAY-010": "请核对参数与账户归属；若持续失败请携带 requestId 排障。",
  };
  const en: Record<string, string> = {
    "PAY-001": "Check DID public key, signed payload, and signature timestamp (within 5 minutes).",
    "PAY-002": "Review authorize rules: single/day limits and merchant whitelist.",
    "PAY-003": "Top up balance or reduce amount, then retry.",
    "PAY-006": "Review risk settings (blocked merchants / amount limits) and adjust policy.",
    "PAY-007": "Channel timeout. Keep requestId and retry later.",
    "PAY-008": "Ensure a unique, non-reused Idempotency-Key is sent.",
    "PAY-010": "Validate request parameters and ownership; include requestId for troubleshooting.",
  };
  if (locale === "en-US") {
    return en[c] ?? "Try retrying with a new request and keep requestId for troubleshooting.";
  }
  return zh[c] ?? "建议重试并保留 requestId 供排障使用。";
}
