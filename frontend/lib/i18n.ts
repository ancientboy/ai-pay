export type Locale = "zh-CN" | "en-US";

interface DictObject {
  [key: string]: string | DictObject;
}

const messages: Record<Locale, DictObject> = {
  "zh-CN": {
    common: {
      close: "关闭",
      create: "创建",
      save: "保存",
      loading: "加载中...",
      success: "成功",
      failed: "失败",
      noData: "暂无数据",
      environment: "环境",
      local: "本地",
      createdAt: "创建时间",
      notAvailable: "暂无",
      yes: "是",
      no: "否",
      offline: "离线",
      ok: "正常",
    },
    nav: {
      dashboard: "仪表盘",
      agents: "Agent 管理",
      authorize: "授权规则",
      recharge: "充值",
      transactions: "交易",
      developer: "开发者中心",
      settings: "设置",
    },
    app: {
      title: "AI 支付控制台",
      subtitle: "MVP 运营后台",
      controlCenter: "控制中心",
      aiNative: "AI 原生支付",
      language: "语言",
    },
    login: {
      title: "AI Pay 登录",
      subtitle: "MVP 测试登录，可后续替换为真实认证",
      username: "用户名",
      password: "密码",
      submit: "登录",
      submitting: "登录中...",
      failed: "登录失败，请重试",
      empty: "用户名和密码不能为空",
    },
    dashboard: {
      title: "仪表盘",
      subtitle: "AI 支付运营总览",
      totalBalance: "总余额 (GUSD)",
      todaySpend: "今日支出 (GUSD)",
      successRate: "支付成功率",
      openAlerts: "待处理告警",
      health: "健康检查",
      readiness: "就绪状态",
      status: "状态",
      requestId: "请求ID",
      timestamp: "时间戳",
      ready: "就绪",
      trend: "趋势",
    },
    agents: {
      title: "Agent 管理",
      subtitle: "管理 Agent DID 与支付账户",
      createPlaceholder: "did:gusd:agent:your_agent_name",
      create: "创建 Agent",
      noAgents: "暂无 Agent，请先创建",
      did: "Agent DID",
      status: "状态",
      va: "VA 账户",
      vaCardNo: "VA 卡号",
      wallet: "钱包地址",
      balance: "余额",
      detail: "Agent 详情",
      createSuccess: "Agent 创建成功",
      didInvalid: "Agent DID 必须以 did: 开头",
    },
    authorize: {
      title: "授权规则",
      subtitle: "配置支付限额与商户白名单",
      agentDid: "Agent DID",
      singleLimit: "单笔限额 (GUSD)",
      dailyLimit: "日限额 (GUSD)",
      whitelist: "商户白名单（逗号分隔）",
      save: "保存规则",
      saveSuccess: "授权规则保存成功",
    },
    recharge: {
      title: "充值",
      subtitle: "提交充值并查看到账状态",
      newRecharge: "发起充值",
      va: "VA 账户 ID",
      vaCardNo: "VA 卡号",
      amount: "金额 (GUSD)",
      submit: "提交充值",
      recent: "最近充值记录",
      detail: "充值详情",
      rechargeId: "充值单号",
      settled: "充值已到账",
      noRecords: "暂无充值记录",
    },
    transactions: {
      title: "交易",
      subtitle: "查看支付状态、手续费与追踪信息",
      createPayment: "发起支付",
      queryTools: "查询工具",
      payerDid: "付款方 DID",
      merchantId: "商户 ID",
      amount: "金额",
      pay: "支付",
      queryStatus: "查状态",
      queryBalance: "查余额",
      queryLedger: "查流水",
      txId: "交易 ID",
      fee: "手续费",
      status: "状态",
      allStatus: "全部状态",
      searchPlaceholder: "搜索交易ID/状态",
      noData: "暂无交易数据",
      page: "页",
      records: "条记录",
      prev: "上一页",
      next: "下一页",
      detail: "交易详情",
      paySuccess: "支付成功",
    },
    status: {
      active: "启用",
      settled: "已结算",
      failed: "失败",
      pending: "处理中",
      unknown: "未知",
    },
    developer: {
      title: "开发者中心",
      subtitle: "管理 API Key 与 Webhook",
      apiKeys: "API Keys",
      keyName: "Key 名称",
      keyNameRequired: "API key 名称不能为空",
      keyCreated: "API key 已创建",
      noKeys: "暂无 API key",
      webhooks: "Webhook",
      webhookUrl: "Webhook URL",
      webhookEvent: "事件名称",
      webhookUrlRequired: "Webhook URL 不能为空",
      webhookCreated: "Webhook 已创建",
      noWebhooks: "暂无 Webhook",
      addWebhook: "新增 Webhook",
    },
    settings: {
      title: "设置",
      subtitle: "配置 API 地址与安全参数",
      apiEndpoint: "API 地址",
      securityNotes: "安全提示",
      save: "保存",
      useEnv: "使用环境默认值",
      saved: "API 地址已保存",
      reverted: "已恢复环境默认地址",
      currentMode: "当前模式",
      runtimeOverride: "运行时覆盖",
      envDefault: "环境默认",
      notes: {
        idempotency: "支付请求必须携带 Idempotency-Key",
        timestamp: "签名时间戳建议控制在 5 分钟以内",
        requestId: "排障时优先按 requestId 追踪",
      },
    },
    validation: {
      agentDidEmpty: "Agent DID 不能为空",
      agentDidFormat: "Agent DID 必须以 did: 开头",
      amountFormat: "金额格式错误",
      amountPositive: "金额必须大于 0",
      whitelistMin: "至少一个白名单商户",
      merchantRequired: "商户 ID 必填",
      vaRequired: "VA 账户必填",
      rechargeTargetRequired: "VA 账户或 VA 卡号至少填写一个",
    },
  },
  "en-US": {
    common: {
      close: "Close",
      create: "Create",
      save: "Save",
      loading: "Loading...",
      success: "Success",
      failed: "Failed",
      noData: "No data",
      environment: "Environment",
      local: "Local",
      createdAt: "Created At",
      notAvailable: "N/A",
      yes: "Yes",
      no: "No",
      offline: "Offline",
      ok: "OK",
    },
    nav: {
      dashboard: "Dashboard",
      agents: "Agents",
      authorize: "Authorize Rules",
      recharge: "Recharge",
      transactions: "Transactions",
      developer: "Developer Center",
      settings: "Settings",
    },
    app: {
      title: "AI Pay Console",
      subtitle: "MVP Operations Panel",
      controlCenter: "Control Center",
      aiNative: "AI-native Payments",
      language: "Language",
    },
    login: {
      title: "AI Pay Login",
      subtitle: "MVP login for testing, replaceable with real auth later",
      username: "Username",
      password: "Password",
      submit: "Login",
      submitting: "Logging in...",
      failed: "Login failed, please retry",
      empty: "Username and password are required",
    },
    dashboard: {
      title: "Dashboard",
      subtitle: "Overview for AI payment operations",
      totalBalance: "Total Balance (GUSD)",
      todaySpend: "Today Spend (GUSD)",
      successRate: "Payment Success Rate",
      openAlerts: "Open Alerts",
      health: "Health Check",
      readiness: "Readiness",
      status: "Status",
      requestId: "Request ID",
      timestamp: "Timestamp",
      ready: "Ready",
      trend: "Trend",
    },
    agents: {
      title: "Agents",
      subtitle: "Manage Agent DID and linked payment accounts",
      createPlaceholder: "did:gusd:agent:your_agent_name",
      create: "Create Agent",
      noAgents: "No agents yet. Create your first one.",
      did: "Agent DID",
      status: "Status",
      va: "VA Account",
      vaCardNo: "VA Card No",
      wallet: "Wallet",
      balance: "Balance",
      detail: "Agent Detail",
      createSuccess: "Agent created",
      didInvalid: "Agent DID must start with did:",
    },
    authorize: {
      title: "Authorize Rules",
      subtitle: "Configure payment amount limits and merchant whitelist",
      agentDid: "Agent DID",
      singleLimit: "Single Limit (GUSD)",
      dailyLimit: "Daily Limit (GUSD)",
      whitelist: "Merchant Whitelist (comma separated)",
      save: "Save Rule",
      saveSuccess: "Rule saved successfully",
    },
    recharge: {
      title: "Recharge",
      subtitle: "Submit recharge request and review settlement status",
      newRecharge: "New Recharge",
      va: "VA Account ID",
      vaCardNo: "VA Card No",
      amount: "Amount (GUSD)",
      submit: "Submit",
      recent: "Recent Recharges",
      detail: "Recharge Detail",
      rechargeId: "Recharge ID",
      settled: "Recharge settled",
      noRecords: "No records yet.",
    },
    transactions: {
      title: "Transactions",
      subtitle: "Track payment status, fees, and tracing info",
      createPayment: "Create Payment",
      queryTools: "Query Tools",
      payerDid: "Payer DID",
      merchantId: "Merchant ID",
      amount: "Amount",
      pay: "Pay",
      queryStatus: "Query Status",
      queryBalance: "Query Balance",
      queryLedger: "Query Ledger",
      txId: "Transaction ID",
      fee: "Fee",
      status: "Status",
      allStatus: "All Status",
      searchPlaceholder: "Search tx id/status",
      noData: "No transaction data yet.",
      page: "Page",
      records: "records",
      prev: "Prev",
      next: "Next",
      detail: "Transaction Detail",
      paySuccess: "Payment success",
    },
    status: {
      active: "Active",
      settled: "Settled",
      failed: "Failed",
      pending: "Pending",
      unknown: "Unknown",
    },
    developer: {
      title: "Developer Center",
      subtitle: "Manage API keys and webhooks",
      apiKeys: "API Keys",
      keyName: "Key name",
      keyNameRequired: "API key name required",
      keyCreated: "API key created",
      noKeys: "No API keys",
      webhooks: "Webhooks",
      webhookUrl: "Webhook URL",
      webhookEvent: "Event name",
      webhookUrlRequired: "Webhook URL required",
      webhookCreated: "Webhook created",
      noWebhooks: "No webhooks",
      addWebhook: "Add Webhook",
    },
    settings: {
      title: "Settings",
      subtitle: "Configure API endpoint and security preferences",
      apiEndpoint: "API Endpoint",
      securityNotes: "Security Notes",
      save: "Save",
      useEnv: "Use Env Default",
      saved: "API endpoint saved",
      reverted: "Reverted to environment endpoint",
      currentMode: "Current mode",
      runtimeOverride: "Runtime Override",
      envDefault: "Env Default",
      notes: {
        idempotency: "Always pass Idempotency-Key for payment requests",
        timestamp: "Sign requests with timestamp within 5 minutes",
        requestId: "Trace failures by requestId in details",
      },
    },
    validation: {
      agentDidEmpty: "Agent DID is required",
      agentDidFormat: "Agent DID must start with did:",
      amountFormat: "Invalid amount format",
      amountPositive: "Amount must be greater than 0",
      whitelistMin: "At least one merchant is required",
      merchantRequired: "Merchant ID is required",
      vaRequired: "VA account is required",
      rechargeTargetRequired: "Either VA account or VA card number is required",
    },
  },
};

export function normalizeLocale(locale?: string | null): Locale {
  if (!locale) {
    return "zh-CN";
  }
  return locale === "en-US" ? "en-US" : "zh-CN";
}

export function localeFromAcceptLanguage(acceptLanguage?: string | null): Locale {
  const primary = (acceptLanguage ?? "")
    .split(",")[0]
    ?.trim()
    .toLowerCase();
  if (primary?.startsWith("en")) {
    return "en-US";
  }
  return "zh-CN";
}

export function t(locale: Locale, key: string): string {
  const segments = key.split(".");
  let node: string | DictObject | undefined = messages[locale];
  for (const seg of segments) {
    if (!node || typeof node === "string") {
      return key;
    }
    node = node[seg];
  }
  return typeof node === "string" ? node : key;
}

export function formatStatus(locale: Locale, status?: string | null): string {
  switch ((status ?? "").toUpperCase()) {
    case "ACTIVE":
      return t(locale, "status.active");
    case "SETTLED":
      return t(locale, "status.settled");
    case "FAILED":
      return t(locale, "status.failed");
    case "PENDING":
    case "PROCESSING":
      return t(locale, "status.pending");
    default:
      return status || t(locale, "status.unknown");
  }
}
