import type { Metadata } from "next";
import { LandingHome } from "./landing-home";

export const metadata: Metadata = {
  title: "AI Pay — AI 原生支付与运营平台",
  description:
    "Agent DID、VA 钱包、订阅收款、对账与权限管控；提供公开接入说明与控制台。",
};

export default function Home() {
  return <LandingHome />;
}
