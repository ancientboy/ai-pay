type AgentKeyRecord = {
  publicKey: string;
  privateKey: string;
};

const AGENT_KEYS_STORAGE_KEY = "ai-pay.agentKeys";

function toBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const b of bytes) {
    binary += String.fromCharCode(b);
  }
  return btoa(binary);
}

function fromBase64(base64: string): ArrayBuffer {
  const binary = atob(base64);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    out[i] = binary.charCodeAt(i);
  }
  return out.buffer;
}

function loadKeyMap(): Record<string, AgentKeyRecord> {
  if (typeof window === "undefined") {
    return {};
  }
  const raw = window.localStorage.getItem(AGENT_KEYS_STORAGE_KEY);
  if (!raw) {
    return {};
  }
  try {
    return JSON.parse(raw) as Record<string, AgentKeyRecord>;
  } catch {
    return {};
  }
}

function saveKeyMap(map: Record<string, AgentKeyRecord>) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(AGENT_KEYS_STORAGE_KEY, JSON.stringify(map));
}

export async function ensureAgentSigningPublicKey(agentDid: string): Promise<string> {
  const map = loadKeyMap();
  const existing = map[agentDid];
  if (existing?.publicKey) {
    return existing.publicKey;
  }
  if (typeof window === "undefined" || !window.crypto?.subtle) {
    throw new Error("crypto subtle unavailable");
  }
  const keyPair = await window.crypto.subtle.generateKey(
    { name: "Ed25519" },
    true,
    ["sign", "verify"],
  );
  const publicRaw = await window.crypto.subtle.exportKey("raw", keyPair.publicKey);
  const privatePkcs8 = await window.crypto.subtle.exportKey("pkcs8", keyPair.privateKey);
  map[agentDid] = {
    publicKey: toBase64(publicRaw),
    privateKey: toBase64(privatePkcs8),
  };
  saveKeyMap(map);
  return map[agentDid].publicKey;
}

export async function signAgentPayload(agentDid: string, payload: string): Promise<string> {
  const map = loadKeyMap();
  const key = map[agentDid];
  if (!key?.privateKey) {
    throw new Error("agent signing key not found");
  }
  if (typeof window === "undefined" || !window.crypto?.subtle) {
    throw new Error("crypto subtle unavailable");
  }
  const privateKey = await window.crypto.subtle.importKey(
    "pkcs8",
    fromBase64(key.privateKey),
    { name: "Ed25519" },
    false,
    ["sign"],
  );
  const encoded = new TextEncoder().encode(payload);
  const signature = await window.crypto.subtle.sign({ name: "Ed25519" }, privateKey, encoded);
  return toBase64(signature);
}

export function buildPaySignPayload(
  payerDid: string,
  merchantId: string,
  amount: string,
  idempotencyKey: string,
  signTimestamp: string,
): string {
  return `${payerDid}|${merchantId}|${amount}|${idempotencyKey}|${signTimestamp}`;
}
