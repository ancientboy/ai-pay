import { createHash } from "crypto";
import { promises as fs } from "fs";
import path from "path";

type StoredUser = {
  username: string;
  passwordHash: string;
  role: string;
  disabled?: boolean;
  onboarding?: {
    dismissed?: boolean;
    completedSteps?: string[];
    updatedAt?: string;
  };
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
  if (hit.disabled) {
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
    disabled: false,
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
    disabled: !!hit.disabled,
    onboarding: {
      dismissed: !!hit.onboarding?.dismissed,
      completedSteps: Array.isArray(hit.onboarding?.completedSteps) ? hit.onboarding?.completedSteps : [],
      updatedAt: hit.onboarding?.updatedAt || hit.createdAt,
    },
    createdAt: hit.createdAt,
  };
}

export async function listStoredUsers() {
  const store = await readStore();
  return store.users
    .map((u) => ({
      username: u.username,
      role: u.role || "operator",
      disabled: !!u.disabled,
      createdAt: u.createdAt,
    }))
    .sort((a, b) => a.username.localeCompare(b.username));
}

export async function updateStoredUser(input: {
  username: string;
  role?: string;
  password?: string;
  disabled?: boolean;
}) {
  const uname = normalizeUsername(input.username);
  if (!uname) {
    return { ok: false as const, code: "AUTH-001" };
  }
  const store = await readStore();
  const idx = store.users.findIndex((u) => normalizeUsername(u.username) === uname);
  if (idx < 0) {
    return { ok: false as const, code: "AUTH-008" };
  }
  if (typeof input.role === "string" && input.role.trim()) {
    store.users[idx].role = input.role.trim();
  }
  if (typeof input.disabled === "boolean") {
    store.users[idx].disabled = input.disabled;
  }
  if (typeof input.password === "string" && input.password.trim()) {
    if (input.password.trim().length < 6) {
      return { ok: false as const, code: "AUTH-006" };
    }
    store.users[idx].passwordHash = hashPassword(input.password.trim());
  }
  await writeStore(store);
  return {
    ok: true as const,
    user: {
      username: store.users[idx].username,
      role: store.users[idx].role || "operator",
      disabled: !!store.users[idx].disabled,
      onboarding: {
        dismissed: !!store.users[idx].onboarding?.dismissed,
        completedSteps: Array.isArray(store.users[idx].onboarding?.completedSteps)
          ? store.users[idx].onboarding?.completedSteps
          : [],
        updatedAt: store.users[idx].onboarding?.updatedAt || store.users[idx].createdAt,
      },
      createdAt: store.users[idx].createdAt,
    },
  };
}

export async function setStoredUserStatus(username: string, status: "active" | "disabled") {
  const result = await updateStoredUser({ username, disabled: status === "disabled" });
  if (!result.ok) {
    return false;
  }
  return true;
}

export async function resetStoredUserPassword(username: string, password: string) {
  const result = await updateStoredUser({ username, password });
  if (!result.ok) {
    return { ok: false as const, code: result.code };
  }
  return { ok: true as const, user: result.user };
}

export async function getStoredUserOnboarding(username: string) {
  const user = await findStoredUserByUsername(username);
  if (!user) {
    return null;
  }
  const completed = new Set(user.onboarding?.completedSteps ?? []);
  return {
    username: user.username,
    dismissed: !!user.onboarding?.dismissed,
    agent: completed.has("agent"),
    kyc: completed.has("kyc"),
    recharge: completed.has("recharge"),
    authorize: completed.has("authorize"),
    pay: completed.has("pay"),
    selfhosted: completed.has("selfhosted"),
    updatedAt: user.onboarding?.updatedAt || user.createdAt,
  };
}

export async function updateStoredUserOnboarding(input: {
  username: string;
  dismissed?: boolean;
  completedStep?: string;
}) {
  const uname = normalizeUsername(input.username);
  if (!uname) {
    return { ok: false as const, code: "AUTH-001" };
  }
  const store = await readStore();
  const idx = store.users.findIndex((u) => normalizeUsername(u.username) === uname);
  if (idx < 0) {
    return { ok: false as const, code: "AUTH-008" };
  }
  const current = store.users[idx].onboarding ?? { dismissed: false, completedSteps: [] as string[] };
  const completedSteps = new Set(Array.isArray(current.completedSteps) ? current.completedSteps : []);
  if (input.completedStep && input.completedStep.trim()) {
    completedSteps.add(input.completedStep.trim());
  }
  store.users[idx].onboarding = {
    dismissed: typeof input.dismissed === "boolean" ? input.dismissed : !!current.dismissed,
    completedSteps: Array.from(completedSteps),
    updatedAt: new Date().toISOString(),
  };
  await writeStore(store);
  const latest = await getStoredUserOnboarding(uname);
  return { ok: true as const, onboarding: latest };
}
