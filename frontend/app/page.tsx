import type { Metadata } from "next";
import { LandingHome } from "./landing-home";

export const metadata: Metadata = {
  title: "AgentTrust Pay 可信付 — Agent 原生可信支付",
  description:
    "AgentTrust Pay（可信付）：面向 Agent 的 DID、签名支付、VA 资金与授权治理；公开接入说明与控制台。",
};

export default function Home() {
  return <LandingHome />;
}
