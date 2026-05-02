import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "接入说明 | AI Pay",
  description: "Agent 接入、VA 充值与支付 API 的小白向说明；无需登录即可阅读。",
};

export default function DocsIntegrationLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
