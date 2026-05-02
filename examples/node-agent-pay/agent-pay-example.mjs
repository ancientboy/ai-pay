/**
 * Minimal end-to-end example aligned with backend/internal/http/integration_flow_test.go
 *
 * Usage:
 *   export AI_PAY_BASE_URL=http://127.0.0.1:8080
 *   node agent-pay-example.mjs
 */

import crypto from "node:crypto";

const BASE =
  process.env.AI_PAY_BASE_URL?.replace(/\/$/, "") ?? "http://127.0.0.1:8080";

function buildPaySignPayload(payerDid, merchantId, amount, idempotencyKey, ts) {
  return `${payerDid}|${merchantId}|${amount}|${idempotencyKey}|${ts}`;
}

async function postJson(path, body, headers = {}) {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...headers,
    },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    json = null;
  }
  if (!res.ok || !json || json.code !== "0") {
    throw new Error(
      `${path} failed: ${res.status} ${text.slice(0, 500)}`,
    );
  }
  return json.data ?? json;
}

async function getJson(path) {
  const res = await fetch(`${BASE}${path}`);
  const text = await res.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    json = null;
  }
  if (!res.ok || !json || json.code !== "0") {
    throw new Error(`${path} failed: ${res.status} ${text.slice(0, 500)}`);
  }
  return json.data ?? json;
}

const agentDid = `did:gusd:agent:node_${Date.now()}`;
const { privateKey, publicKey } = crypto.generateKeyPairSync("ed25519");
const jwk = publicKey.export({ format: "jwk" });
const rawPub = Buffer.from(jwk.x, "base64url");
const didPubKey = rawPub.toString("base64");

const ts = new Date().toISOString();
const idemRegister = `idem-reg-${Date.now()}`;
const idemRecharge = `idem-rch-${Date.now()}`;
const idemPay = `idem-pay-${Date.now()}`;

console.log("Base URL:", BASE);
console.log("Agent DID:", agentDid);

await postJson(
  "/agent/did/register",
  { agentDid, didPubKey },
  { "Idempotency-Key": idemRegister },
);

const account = await postJson("/account/create", { agentDid });
const vaId = account.VAAccountID ?? account.vaAccountId;
if (!vaId) {
  throw new Error("missing VA account id in create response");
}

await postJson(
  "/fund/recharge",
  { vaAccountId: vaId, amount: "100" },
  { "Idempotency-Key": idemRecharge },
);

await postJson("/authorize/payment/set", {
  agentDid,
  singleLimit: "50",
  dailyLimit: "200",
  whitelist: ["m1"],
});

const signPayload = Buffer.from(
  buildPaySignPayload(agentDid, "m1", "10", idemPay, ts),
  "utf8",
);
const signature = crypto.sign(null, signPayload, privateKey);
const signatureB64 = signature.toString("base64");

const payData = await postJson(
  "/payment/x402/pay",
  {
    payerDid: agentDid,
    merchantId: "m1",
    amount: "10",
    signature: signatureB64,
  },
  {
    "Idempotency-Key": idemPay,
    "X-Sign-Timestamp": ts,
  },
);

const txId =
  payData.transactionId ?? payData.TransactionID ?? payData.transaction_id;
if (!txId) {
  console.log("Pay response:", payData);
  throw new Error("missing transaction id");
}

const status = await getJson(
  `/payment/status/query?transactionId=${encodeURIComponent(txId)}`,
);

console.log("transactionId:", txId);
console.log("status query:", JSON.stringify(status, null, 2));
console.log("OK");
