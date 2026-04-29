import { promises as fs } from "fs";
import path from "path";

export type AdminAuditAction =
  | "admin.user.create"
  | "admin.user.enable"
  | "admin.user.disable"
  | "admin.user.reset_password";

export type AdminAuditLog = {
  id: string;
  actor: string;
  action: AdminAuditAction;
  targetUsername: string;
  detail?: Record<string, unknown>;
  createdAt: string;
};

export type AdminAuditQuery = {
  actor?: string;
  action?: AdminAuditAction | "";
  targetUsername?: string;
  limit?: number;
  offset?: number;
};

type AdminAuditStore = {
  logs: AdminAuditLog[];
};

const STORE_DIR = path.join(process.cwd(), ".local-data");
const STORE_PATH = path.join(STORE_DIR, "admin-audit-logs.json");

async function ensureStore() {
  await fs.mkdir(STORE_DIR, { recursive: true });
  try {
    await fs.access(STORE_PATH);
  } catch {
    const init: AdminAuditStore = { logs: [] };
    await fs.writeFile(STORE_PATH, JSON.stringify(init, null, 2), "utf-8");
  }
}

async function readStore(): Promise<AdminAuditStore> {
  await ensureStore();
  const raw = await fs.readFile(STORE_PATH, "utf-8");
  try {
    const parsed = JSON.parse(raw) as AdminAuditStore;
    if (!Array.isArray(parsed.logs)) {
      return { logs: [] };
    }
    return parsed;
  } catch {
    return { logs: [] };
  }
}

async function writeStore(store: AdminAuditStore) {
  await ensureStore();
  await fs.writeFile(STORE_PATH, JSON.stringify(store, null, 2), "utf-8");
}

export async function appendAdminAuditLog(input: {
  actor: string;
  action: AdminAuditAction;
  targetUsername: string;
  detail?: Record<string, unknown>;
}) {
  const store = await readStore();
  const log: AdminAuditLog = {
    id: `audit-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    actor: input.actor.trim().toLowerCase() || "unknown",
    action: input.action,
    targetUsername: input.targetUsername.trim().toLowerCase(),
    detail: input.detail ?? {},
    createdAt: new Date().toISOString(),
  };
  store.logs.unshift(log);
  if (store.logs.length > 500) {
    store.logs = store.logs.slice(0, 500);
  }
  await writeStore(store);
  return log;
}

export async function listAdminAuditLogs(input?: AdminAuditQuery) {
  const safeLimit =
    typeof input?.limit === "number" && Number.isFinite(input.limit) && input.limit > 0
      ? Math.min(input.limit, 200)
      : 50;
  const safeOffset =
    typeof input?.offset === "number" && Number.isFinite(input.offset) && input.offset >= 0
      ? Math.floor(input.offset)
      : 0;
  const actor = (input?.actor ?? "").trim().toLowerCase();
  const action = (input?.action ?? "").trim().toLowerCase();
  const targetUsername = (input?.targetUsername ?? "").trim().toLowerCase();
  const store = await readStore();
  const filtered = store.logs.filter((log) => {
    if (actor && !log.actor.includes(actor)) {
      return false;
    }
    if (action && log.action !== action) {
      return false;
    }
    if (targetUsername && !log.targetUsername.includes(targetUsername)) {
      return false;
    }
    return true;
  });
  return {
    items: filtered.slice(safeOffset, safeOffset + safeLimit),
    total: filtered.length,
    limit: safeLimit,
    offset: safeOffset,
  };
}
