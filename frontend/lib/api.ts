export type HealthResponse = {
  code: string;
  data: {
    status?: string;
    ready?: boolean;
    requestId: string;
    time: string;
  };
};

export type DashboardMetrics = {
  totalBalance: number;
  todaySpend: number;
  paymentSuccessRate: number;
  alertCount: number;
};

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`API request failed: ${response.status}`);
  }
  return (await response.json()) as T;
}

export async function getHealth(): Promise<HealthResponse> {
  return fetchJSON<HealthResponse>("/health");
}

export async function getReady(): Promise<HealthResponse> {
  return fetchJSON<HealthResponse>("/ready");
}

export async function getDashboardMetrics(): Promise<DashboardMetrics> {
  const payload = await fetchJSON<{ code: string; data: DashboardMetrics }>(
    "/metrics/overview",
  );
  return payload.data;
}
