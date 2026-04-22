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
};

export class ApiClientError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
    this.name = "ApiClientError";
  }
}

export function toReadableError(err: unknown, locale: "zh-CN" | "en-US" = "zh-CN"): string {
  const map = locale === "en-US" ? ERROR_MAP_EN : ERROR_MAP;
  if (err instanceof ApiClientError) {
    const friendly = map[err.code];
    if (friendly) {
      return `${friendly}（${err.code}）`;
    }
    return `${err.message}（${err.code}）`;
  }
  if (err instanceof Error) {
    return err.message;
  }
  return "未知错误";
}
