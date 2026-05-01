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
  "AUTH-004": "用户名至少 3 位，仅支持字母/数字/下划线/中划线",
  "AUTH-005": "密码至少 8 位",
  "AUTH-006": "用户名已存在",
  "AUTH-007": "该账号已禁用，请联系管理员",
  "AUTH-008": "仅管理员可执行此操作",
  "AUTH-009": "请求的用户不存在",
  "AUTH-010": "管理员账号不能禁用",
  "AUTH-011": "管理员账号角色不可变更",
  "AUTH-012": "当前角色不允许执行此写操作",
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
  "AUTH-004": "Username must be >=3 chars and use letters/digits/_/- only",
  "AUTH-005": "Password must be at least 8 characters",
  "AUTH-006": "Username already exists",
  "AUTH-007": "Account is disabled, contact administrator",
  "AUTH-008": "Administrator privileges required",
  "AUTH-009": "User not found",
  "AUTH-010": "Administrator account cannot be disabled",
  "AUTH-011": "Administrator role cannot be changed",
  "AUTH-012": "Current role is not allowed to perform this write action",
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
