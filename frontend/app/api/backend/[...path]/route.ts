import { NextRequest, NextResponse } from "next/server";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";

function resolveBaseURL(request: NextRequest) {
  const override = request.headers.get("x-api-base-url");
  if (override && /^https?:\/\//.test(override)) {
    return override;
  }
  return API_BASE_URL;
}

async function proxy(request: NextRequest, path: string[]) {
  const baseURL = resolveBaseURL(request);
  const url = new URL(`${baseURL}/${path.join("/")}`);
  request.nextUrl.searchParams.forEach((value, key) => {
    url.searchParams.set(key, value);
  });

  const headers = new Headers();
  ["content-type", "idempotency-key", "x-sign-timestamp", "x-request-id"].forEach(
    (name) => {
      const value = request.headers.get(name);
      if (value) {
        headers.set(name, value);
      }
    },
  );

  const body =
    request.method === "GET" || request.method === "HEAD"
      ? undefined
      : await request.text();

  const response = await fetch(url.toString(), {
    method: request.method,
    headers,
    body,
    cache: "no-store",
  });

  const text = await response.text();
  return new NextResponse(text, {
    status: response.status,
    headers: {
      "content-type": response.headers.get("content-type") ?? "application/json",
      "x-request-id": response.headers.get("x-request-id") ?? "",
    },
  });
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  return proxy(request, path);
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  return proxy(request, path);
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  return proxy(request, path);
}
