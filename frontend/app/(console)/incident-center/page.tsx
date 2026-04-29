"use client";

import { useLocale } from "@/components/locale-provider";

type IncidentItem = {
  code: string;
  level: "self_service" | "ops_required" | "tech_required";
  scenario: string;
  action: string;
  owner: string;
};

function levelLabel(level: IncidentItem["level"], isEN: boolean) {
  if (level === "self_service") {
    return isEN ? "Self-service" : "用户可自助";
  }
  if (level === "ops_required") {
    return isEN ? "Ops required" : "需运营介入";
  }
  return isEN ? "Engineering required" : "需技术介入";
}

export default function IncidentCenterPage() {
  const { locale } = useLocale();
  const isEN = locale === "en-US";
  const items: IncidentItem[] = [
    {
      code: "PAY-001",
      level: "self_service",
      scenario: isEN ? "Signature / timestamp invalid" : "签名或时间戳无效",
      action: isEN
        ? "Re-create DID key pair or auth session, then retry."
        : "重建 DID 密钥或签名会话后重试。",
      owner: isEN ? "User / Agent operator" : "用户/Agent 运营",
    },
    {
      code: "PAY-002",
      level: "ops_required",
      scenario: isEN ? "Authorize rule rejected" : "授权规则拒绝",
      action: isEN
        ? "Adjust single/day limits and merchant whitelist."
        : "调整单笔/日限额与商户白名单。",
      owner: isEN ? "Operations" : "运营",
    },
    {
      code: "PAY-003",
      level: "self_service",
      scenario: isEN ? "Insufficient balance" : "余额不足",
      action: isEN ? "Recharge VA account first." : "先完成 VA 账户充值。",
      owner: isEN ? "User" : "用户",
    },
    {
      code: "PAY-006",
      level: "ops_required",
      scenario: isEN ? "Risk block" : "风控拦截",
      action: isEN
        ? "Review risk threshold / blocklist and approve policy changes."
        : "检查风控阈值和拦截名单，并审批策略放行。",
      owner: isEN ? "Risk Ops" : "风控运营",
    },
    {
      code: "PAY-007",
      level: "tech_required",
      scenario: isEN ? "Channel timeout" : "通道超时",
      action: isEN
        ? "Check provider health, retry policy, and route fallback."
        : "检查通道健康、重试策略和路由降级。",
      owner: isEN ? "Engineering + Ops" : "技术+运营",
    },
    {
      code: "PAY-010",
      level: "tech_required",
      scenario: isEN ? "Unknown/busy/invalid request" : "系统繁忙或参数异常",
      action: isEN
        ? "Use requestId to inspect logs and trace backend failures."
        : "使用 requestId 排查后端日志并定位失败链路。",
      owner: isEN ? "Engineering" : "技术",
    },
  ];

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">
          {isEN ? "Payment Incident Center" : "支付失败分级处置中心"}
        </h2>
        <p className="mt-1 text-sm text-slate-400">
          {isEN
            ? "Standardize troubleshooting paths by error code severity."
            : "按错误码严重级别标准化处置路径。"}
        </p>
      </div>
      <div className="rounded-xl border border-blue-700/50 bg-blue-950/30 p-4 text-sm text-blue-100">
        <p className="font-medium">{isEN ? "RequestId linkage guide" : "requestId 联动指引"}</p>
        <ol className="mt-2 list-decimal space-y-1 pl-5 text-xs text-blue-200">
          <li>
            {isEN
              ? "When payment fails, copy requestId from error panel in Recharge/Transactions."
              : "支付失败时先从充值/交易页错误面板复制 requestId。"}
          </li>
          <li>
            {isEN
              ? "Match requestId with backend logs to find exact failure stage."
              : "在后端日志中检索 requestId，定位具体失败阶段。"}
          </li>
          <li>
            {isEN
              ? "Apply actions below by error code level, then retry with new idempotency key."
              : "按下方错误码分级执行处置后，使用新幂等键重试。"}
          </li>
        </ol>
      </div>
      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-800 text-left text-slate-400">
              <th className="pb-2">Code</th>
              <th className="pb-2">{isEN ? "Level" : "级别"}</th>
              <th className="pb-2">{isEN ? "Scenario" : "场景"}</th>
              <th className="pb-2">{isEN ? "Action" : "处置动作"}</th>
              <th className="pb-2">{isEN ? "Owner" : "责任方"}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.code} className="border-b border-slate-800/60 align-top">
                <td className="py-3 font-mono text-xs text-slate-200">{item.code}</td>
                <td className="py-3 text-xs text-slate-300">{levelLabel(item.level, isEN)}</td>
                <td className="py-3 text-xs text-slate-300">{item.scenario}</td>
                <td className="py-3 text-xs text-slate-300">{item.action}</td>
                <td className="py-3 text-xs text-slate-400">{item.owner}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
