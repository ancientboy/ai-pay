import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "接入说明 | AgentTrust Pay 可信付",
  description: "AgentTrust Pay：Agent 接入、VA 充值与签名支付 API；无需登录即可阅读。",
};

export default function DocsIntegrationLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
