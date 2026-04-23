const SESSION_SECRET_FALLBACK = "dev-only-session-secret-change-me";
const SIGNATURE_ALGORITHM = { name: "HMAC", hash: "SHA-256" } as const;

export const SESSION_COOKIE_NAME = "ai_pay_session";
export const SESSION_TTL_SECONDS = 60 * 60 * 8;

type SessionClaims = {
  sub: string;
  iat: number;
  exp: number;
};

let hmacKeyPromise: Promise<CryptoKey> | null = null;

function getSessionSecret() {
  const secret = process.env.AI_PAY_SESSION_SECRET?.trim();
  return secret || SESSION_SECRET_FALLBACK;
}

function toBase64Url(input: string | Uint8Array) {
  const bytes =
    typeof input === "string" ? new TextEncoder().encode(input) : input;
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function fromBase64Url(base64Url: string) {
  const normalized = base64Url.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
  const binary = atob(padded);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    out[i] = binary.charCodeAt(i);
  }
  return out;
}

async function getHmacKey() {
  if (!hmacKeyPromise) {
    hmacKeyPromise = crypto.subtle.importKey(
      "raw",
      new TextEncoder().encode(getSessionSecret()),
      SIGNATURE_ALGORITHM,
      false,
      ["sign", "verify"],
    );
  }
  return hmacKeyPromise;
}

async function signPayload(payloadSegment: string) {
  const key = await getHmacKey();
  const digest = await crypto.subtle.sign(
    SIGNATURE_ALGORITHM.name,
    key,
    new TextEncoder().encode(payloadSegment),
  );
  return toBase64Url(new Uint8Array(digest));
}

function safeEqual(a: string, b: string) {
  if (a.length !== b.length) {
    return false;
  }
  let diff = 0;
  for (let i = 0; i < a.length; i += 1) {
    diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }
  return diff === 0;
}

export async function createSessionToken(subject: string) {
  const now = Math.floor(Date.now() / 1000);
  const claims: SessionClaims = {
    sub: subject,
    iat: now,
    exp: now + SESSION_TTL_SECONDS,
  };
  const payloadSegment = toBase64Url(JSON.stringify(claims));
  const signature = await signPayload(payloadSegment);
  return `${payloadSegment}.${signature}`;
}

export async function verifySessionToken(token?: string) {
  if (!token) {
    return false;
  }
  const [payloadSegment, signature] = token.split(".");
  if (!payloadSegment || !signature) {
    return false;
  }

  const expected = await signPayload(payloadSegment);
  if (!safeEqual(signature, expected)) {
    return false;
  }

  try {
    const claims = JSON.parse(
      new TextDecoder().decode(fromBase64Url(payloadSegment)),
    ) as SessionClaims;
    const now = Math.floor(Date.now() / 1000);
    if (!claims.sub || !claims.iat || !claims.exp) {
      return false;
    }
    if (claims.exp <= now) {
      return false;
    }
    if (claims.iat > now+ 60) {
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

export function sessionCookieOptions(maxAge = SESSION_TTL_SECONDS) {
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge,
  };
}
