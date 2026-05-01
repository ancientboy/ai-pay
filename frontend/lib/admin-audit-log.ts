import { mkdir, readFile, writeFile } from "fs/promises";
import path from "path";

export type AdminAuditAction =
  | "admin.user.create"
  | "admin.user.disable"
  | "admin.user.enable"
  | "admin.user.reset_password"
  | "admin.user.set_role"
  | "admin.user.set_plan";

type AdminAuditRecord = {
  id: string;
  actor: string;
  action: AdminAuditAction;
  targetUsername: string;
  detail?: Record<string, unknown>;
  createdAt: string;
};

type AdminAuditStore = {
  logs: AdminAuditRecord[];
};

const DATA_DIR = path.join(process.cwd(), ".local-data");
const AUDIT_FILE = path.join(DATA_DIR, "admin-audit-logs.json");

async function readStore(): Promise<AdminAuditStore> {
  try {
    const raw = await readFile(AUDIT_FILE, "utf8");
    const parsed = JSON.parse(raw) as AdminAuditStore;
    return { logs: Array.isArray(parsed.logs) ? parsed.logs : [] };
  } catch {
    return { logs: [] };
  }
}

async function writeStore(store: AdminAuditStore) {
  await mkdir(DATA_DIR, { recursive: true });
  await writeFile(AUDIT_FILE, JSON.stringify(store, null, 2));
}

function makeId() {
  return `audit-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export async function appendAdminAuditLog(input: {
  actor: string;
  action: AdminAuditAction;
  targetUsername: string;
  detail?: Record<string, unknown>;
}) {
  const store = await readStore();
  store.logs.unshift({
    id: makeId(),
    actor: input.actor,
    action: input.action,
    targetUsername: input.targetUsername,
    detail: input.detail,
    createdAt: new Date().toISOString(),
  });
  store.logs = store.logs.slice(0, 500);
  await writeStore(store);
}

export async function listAdminAuditLogs(input?: { limit?: number; offset?: number }) {
  const store = await readStore();
  const offset =
    typeof input?.offset === "number" && Number.isFinite(input.offset) && input.offset >= 0
      ? input.offset
      : 0;
  const limit =
    typeof input?.limit === "number" && Number.isFinite(input.limit) && input.limit > 0
      ? input.limit
      : 50;
  return store.logs.slice(offset, offset + limit);
}
