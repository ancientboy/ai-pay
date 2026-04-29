import { createHash } from "crypto";
import { promises as fs } from "fs";
import path from "path";

type StoredUser = {
  username: string;
  passwordHash: string;
  role: string;
  createdAt: string;
};

type UserStore = {
  users: StoredUser[];
};

const STORE_DIR = path.join(process.cwd(), ".local-data");
const STORE_PATH = path.join(STORE_DIR, "users.json");

function normalizeUsername(username: string) {
  return username.trim().toLowerCase();
}

function hashPassword(rawPassword: string) {
  return createHash("sha256").update(rawPassword).digest("hex");
}

async function ensureStore() {
  await fs.mkdir(STORE_DIR, { recursive: true });
  try {
    await fs.access(STORE_PATH);
  } catch {
    const init: UserStore = { users: [] };
    await fs.writeFile(STORE_PATH, JSON.stringify(init, null, 2), "utf-8");
  }
}

async function readStore(): Promise<UserStore> {
  await ensureStore();
  const raw = await fs.readFile(STORE_PATH, "utf-8");
  try {
    const parsed = JSON.parse(raw) as UserStore;
    if (!Array.isArray(parsed.users)) {
      return { users: [] };
    }
    return parsed;
  } catch {
    return { users: [] };
  }
}

async function writeStore(store: UserStore) {
  await ensureStore();
  await fs.writeFile(STORE_PATH, JSON.stringify(store, null, 2), "utf-8");
}

export async function verifyStoredUser(username: string, password: string) {
  const uname = normalizeUsername(username);
  if (!uname || !password.trim()) {
    return null;
  }
  const store = await readStore();
  const hit = store.users.find((u) => normalizeUsername(u.username) === uname);
  if (!hit) {
    return null;
  }
  if (hit.passwordHash !== hashPassword(password.trim())) {
    return null;
  }
  return { username: hit.username, role: hit.role || "operator" };
}

export async function registerStoredUser(input: {
  username: string;
  password: string;
  role?: string;
}) {
  const uname = normalizeUsername(input.username);
  const pwd = input.password.trim();
  if (uname.length < 3) {
    return { ok: false as const, code: "AUTH-001", message: "username too short" };
  }
  if (pwd.length < 6) {
    return { ok: false as const, code: "AUTH-001", message: "password too short" };
  }
  const store = await readStore();
  const exists = store.users.some((u) => normalizeUsername(u.username) === uname);
  if (exists) {
    return { ok: false as const, code: "AUTH-004", message: "user already exists" };
  }
  store.users.push({
    username: uname,
    passwordHash: hashPassword(pwd),
    role: (input.role || "operator").trim() || "operator",
    createdAt: new Date().toISOString(),
  });
  await writeStore(store);
  return { ok: true as const, user: { username: uname, role: (input.role || "operator").trim() || "operator" } };
}

export async function findStoredUserByUsername(username: string) {
  const uname = normalizeUsername(username);
  if (!uname) {
    return null;
  }
  const store = await readStore();
  const hit = store.users.find((u) => normalizeUsername(u.username) === uname);
  if (!hit) {
    return null;
  }
  return {
    username: hit.username,
    role: hit.role || "operator",
    createdAt: hit.createdAt,
  };
}
