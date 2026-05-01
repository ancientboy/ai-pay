import { createHash } from "crypto";
import { mkdir, readFile, writeFile } from "fs/promises";
import path from "path";

export type UserRole = "admin" | "operator" | "readonly";

export type StoredUser = {
  username: string;
  passwordHash: string;
  role: UserRole;
  createdAt: string;
  disabled?: boolean;
};

type UsersStore = {
  users: StoredUser[];
};

const DATA_DIR = path.join(process.cwd(), ".local-data");
const USERS_FILE = path.join(DATA_DIR, "users.json");

function normalizeUsername(input: string) {
  return input.trim().toLowerCase();
}

function adminUsername() {
  return normalizeUsername(process.env.AI_PAY_ADMIN_USERNAME ?? "admin");
}

function adminPassword() {
  return process.env.AI_PAY_ADMIN_PASSWORD ?? "admin123";
}

export function hashPassword(password: string) {
  return createHash("sha256").update(password).digest("hex");
}

export function verifyPassword(password: string, passwordHash: string) {
  return hashPassword(password) === passwordHash;
}

async function readUsersStore(): Promise<UsersStore> {
  try {
    const raw = await readFile(USERS_FILE, "utf8");
    const parsed = JSON.parse(raw) as UsersStore;
    return { users: Array.isArray(parsed.users) ? parsed.users : [] };
  } catch {
    return { users: [] };
  }
}

async function writeUsersStore(store: UsersStore) {
  await mkdir(DATA_DIR, { recursive: true });
  await writeFile(USERS_FILE, JSON.stringify(store, null, 2));
}

export async function ensureAdminUser() {
  const store = await readUsersStore();
  const uname = adminUsername();
  const idx = store.users.findIndex((u) => normalizeUsername(u.username) === uname);
  const admin: StoredUser = {
    username: uname,
    passwordHash: hashPassword(adminPassword()),
    role: "admin",
    createdAt: new Date().toISOString(),
    disabled: false,
  };
  if (idx >= 0) {
    store.users[idx] = {
      ...store.users[idx],
      username: uname,
      passwordHash: admin.passwordHash,
      role: "admin",
      disabled: false,
    };
  } else {
    store.users.push(admin);
  }
  await writeUsersStore(store);
}

export async function findUserByUsername(username: string) {
  const normalized = normalizeUsername(username);
  const store = await readUsersStore();
  return store.users.find((u) => normalizeUsername(u.username) === normalized);
}

export async function upsertUser(input: { username: string; password: string; role?: UserRole }) {
  const normalized = normalizeUsername(input.username);
  if (!normalized) {
    return { ok: false as const, reason: "invalid_username" as const };
  }
  const store = await readUsersStore();
  if (store.users.some((u) => normalizeUsername(u.username) === normalized)) {
    return { ok: false as const, reason: "exists" as const };
  }
  store.users.push({
    username: normalized,
    passwordHash: hashPassword(input.password),
    role: input.role ?? "operator",
    createdAt: new Date().toISOString(),
    disabled: false,
  });
  await writeUsersStore(store);
  return { ok: true as const };
}

export async function listUsers() {
  const store = await readUsersStore();
  return [...store.users].sort((a, b) => a.username.localeCompare(b.username));
}

export async function updateUserRole(username: string, role: UserRole) {
  const normalized = normalizeUsername(username);
  const store = await readUsersStore();
  const idx = store.users.findIndex((u) => normalizeUsername(u.username) === normalized);
  if (idx < 0) {
    return { ok: false as const, reason: "not_found" as const };
  }
  store.users[idx] = { ...store.users[idx], role };
  await writeUsersStore(store);
  return { ok: true as const };
}

export async function setUserDisabled(username: string, disabled: boolean) {
  const normalized = normalizeUsername(username);
  const store = await readUsersStore();
  const idx = store.users.findIndex((u) => normalizeUsername(u.username) === normalized);
  if (idx < 0) {
    return { ok: false as const, reason: "not_found" as const };
  }
  if (normalizeUsername(store.users[idx].username) === adminUsername()) {
    return { ok: false as const, reason: "protected_admin" as const };
  }
  store.users[idx] = { ...store.users[idx], disabled };
  await writeUsersStore(store);
  return { ok: true as const };
}

export async function resetUserPassword(username: string, password: string) {
  const normalized = normalizeUsername(username);
  const store = await readUsersStore();
  const idx = store.users.findIndex((u) => normalizeUsername(u.username) === normalized);
  if (idx < 0) {
    return { ok: false as const, reason: "not_found" as const };
  }
  store.users[idx] = {
    ...store.users[idx],
    passwordHash: hashPassword(password),
    disabled: false,
  };
  await writeUsersStore(store);
  return { ok: true as const };
}
