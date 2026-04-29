"use client";

import { useQuery } from "@tanstack/react-query";
import { useLocale } from "@/components/locale-provider";
import { checkStablecoinProviderHealth } from "@/lib/console-api";

type Props = {
  providers?: string[];
};

export function EnvReadinessBanner({ providers = ["bridge", "mock"] }: Props) {
  const { locale } = useLocale();
  const isEN = locale === "en-US";

  const bridge = useQuery({
    queryKey: ["provider-health", "bridge"],
    queryFn: () => checkStablecoinProviderHealth("bridge"),
  });
  const mock = useQuery({
    queryKey: ["provider-health", "mock"],
    queryFn: () => checkStablecoinProviderHealth("mock"),
  });

  const list = providers
    .map((p) => {
      if (p === "bridge") {
        return { name: "bridge", healthy: !!bridge.data?.healthy };
      }
      if (p === "mock") {
        return { name: "mock", healthy: !!mock.data?.healthy };
      }
      return null;
    })
    .filter(Boolean) as Array<{ name: string; healthy: boolean }>;

  const hasReal = list.some((x) => x.name !== "mock" && x.healthy);
  const tone = hasReal
    ? "border-emerald-700/60 bg-emerald-950/30 text-emerald-100"
    : "border-amber-700/60 bg-amber-950/30 text-amber-100";

  return (
    <div className={`rounded-xl border p-3 text-xs ${tone}`}>
      <p className="font-medium">
        {isEN ? "Environment readiness" : "环境就绪状态"}
      </p>
      <p className="mt-1">
        {isEN
          ? "Real payment requires at least one healthy non-mock provider."
          : "真实支付至少需要一个可用的非 mock Provider。"}
      </p>
      <ul className="mt-2 space-y-1">
        {list.map((item) => (
          <li key={item.name}>
            {item.name}: {item.healthy ? (isEN ? "healthy" : "可用") : (isEN ? "unhealthy" : "不可用")}
          </li>
        ))}
      </ul>
    </div>
  );
}
