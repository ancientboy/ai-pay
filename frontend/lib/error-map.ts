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

export class ApiClientError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
    this.name = "ApiClientError";
  }
}

export function toReadableError(err: unknown): string {
  if (err instanceof ApiClientError) {
    const friendly = ERROR_MAP[err.code];
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
