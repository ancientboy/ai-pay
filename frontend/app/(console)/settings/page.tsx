"use client";

import { useMemo, useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  createUserAsAdmin,
  getSavedApiBaseURL,
  saveApiBaseURL,
  listAdminUsers,
  listAdminAuditLogs,
  buildAdminAuditExportUrl,
  updateAdminUserStatus,
  resetAdminUserPassword,
  type AdminUserRecord,
  type AdminAuditRecord as AdminUserAuditLog,
} from "@/lib/console-api";

export default function SettingsPage() {
  const { t } = useLocale();
  const { showToast } = useToast();
  const envBaseURL =
    process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";
  const [apiBaseURL, setApiBaseURL] = useState(() => {
    const saved = getSavedApiBaseURL();
    return saved || envBaseURL;
  });

  const usingRuntimeOverride = useMemo(
    () => apiBaseURL.trim() !== "" && apiBaseURL.trim() !== envBaseURL,
    [apiBaseURL, envBaseURL],
  );
  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newRole, setNewRole] = useState<"operator" | "admin" | "readonly">("operator");
  const [creatingUser, setCreatingUser] = useState(false);
  const [users, setUsers] = useState<AdminUserRecord[]>([]);
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [updatingUser, setUpdatingUser] = useState("");
  const [resetUsername, setResetUsername] = useState("");
  const [resetPassword, setResetPassword] = useState("");
  const [resettingPassword, setResettingPassword] = useState(false);
  const [auditLogs, setAuditLogs] = useState<AdminUserAuditLog[]>([]);
  const [loadingAuditLogs, setLoadingAuditLogs] = useState(false);
  const [exportingAuditLogs, setExportingAuditLogs] = useState(false);
  const [auditActorFilter, setAuditActorFilter] = useState("");
  const [auditTargetFilter, setAuditTargetFilter] = useState("");
  const [auditActionFilter, setAuditActionFilter] = useState<
    "" | AdminUserAuditLog["action"]
  >("");
  const [auditOffset, setAuditOffset] = useState(0);
  const auditPageSize = 10;

  async function refreshUsers() {
    setLoadingUsers(true);
    try {
      const items = await listAdminUsers();
      setUsers(items);
    } catch (err) {
      showToast("error", err instanceof Error ? err.message : "load users failed");
    } finally {
      setLoadingUsers(false);
    }
  }

  async function refreshAuditLogs() {
    setLoadingAuditLogs(true);
    try {
      const actionFilter = auditActionFilter.trim() as
        | ""
        | "admin.user.create"
        | "admin.user.enable"
        | "admin.user.disable"
        | "admin.user.reset_password";
      const items = await listAdminAuditLogs({
        limit: auditPageSize,
        offset: auditOffset,
        actor: auditActorFilter.trim(),
        targetUsername: auditTargetFilter.trim(),
        action: actionFilter,
      });
      setAuditLogs(items);
    } catch (err) {
      showToast("error", err instanceof Error ? err.message : "load audit logs failed");
    } finally {
      setLoadingAuditLogs(false);
    }
  }

  async function handleExportAuditCsv() {
    setExportingAuditLogs(true);
    try {
      const url = buildAdminAuditExportUrl({
        actor: auditActorFilter.trim(),
        action: auditActionFilter || undefined,
        targetUsername: auditTargetFilter.trim(),
      });
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `admin-audit-logs-${Date.now()}.csv`;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      showToast("success", "csv exported");
    } catch (err) {
      showToast("error", err instanceof Error ? err.message : "export csv failed");
    } finally {
      setExportingAuditLogs(false);
    }
  }

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("settings.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("settings.subtitle")}</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("settings.apiEndpoint")}</h3>
          <p className="mt-2 text-sm text-slate-400">
            NEXT_PUBLIC_API_BASE_URL
          </p>
          <div className="mt-1 space-y-2">
            <input
              value={apiBaseURL}
              onChange={(e) => setApiBaseURL(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-slate-300"
            />
            <div className="flex gap-2">
              <button
                onClick={() => {
                  saveApiBaseURL(apiBaseURL);
                  showToast("success", t("settings.saved"));
                }}
                className="rounded-md bg-blue-600 px-3 py-1 text-xs text-white"
              >
                {t("settings.save")}
              </button>
              <button
                onClick={() => {
                  saveApiBaseURL("");
                  setApiBaseURL(envBaseURL);
                  showToast("info", t("settings.reverted"));
                }}
                className="rounded-md border border-slate-700 px-3 py-1 text-xs text-slate-200"
              >
                {t("settings.useEnv")}
              </button>
            </div>
            <p className="text-xs text-slate-500">
              {t("settings.currentMode")}: {usingRuntimeOverride ? t("settings.runtimeOverride") : t("settings.envDefault")}
            </p>
          </div>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("settings.securityNotes")}</h3>
          <ul className="mt-2 list-disc space-y-1 pl-4 text-sm text-slate-400">
            <li>{t("settings.notes.idempotency")}</li>
            <li>{t("settings.notes.timestamp")}</li>
            <li>{t("settings.notes.requestId")}</li>
          </ul>
        </div>
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">Admin: Create User</h3>
          <p className="mt-2 text-sm text-slate-400">Create console user accounts manually.</p>
          <div className="mt-3 space-y-2">
            <input
              value={newUsername}
              onChange={(e) => setNewUsername(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200"
              placeholder="username"
            />
            <input
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200"
              placeholder="password"
              type="password"
            />
            <select
              value={newRole}
              onChange={(e) => setNewRole(e.target.value as "operator" | "admin" | "readonly")}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200"
            >
              <option value="operator">operator</option>
              <option value="admin">admin</option>
              <option value="readonly">readonly</option>
            </select>
            <button
              type="button"
              disabled={creatingUser}
              onClick={async () => {
                if (!newUsername.trim() || !newPassword.trim()) {
                  showToast("error", "username/password required");
                  return;
                }
                setCreatingUser(true);
                try {
                  await createUserAsAdmin({
                    username: newUsername.trim(),
                    password: newPassword.trim(),
                    role: newRole,
                  });
                  showToast("success", "user created");
                  setNewUsername("");
                  setNewPassword("");
                  setNewRole("operator");
                  await refreshUsers();
                  await refreshAuditLogs();
                } catch (err) {
                  showToast("error", err instanceof Error ? err.message : "create user failed");
                } finally {
                  setCreatingUser(false);
                }
              }}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white disabled:opacity-60"
            >
              {creatingUser ? "creating..." : "create user"}
            </button>
          </div>
          <div className="mt-3 flex gap-2">
            <button
              type="button"
              onClick={() => {
                refreshUsers();
              }}
              className="rounded-md border border-slate-700 px-3 py-1 text-xs text-slate-200"
            >
              {loadingUsers ? "loading..." : "refresh users"}
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {users.map((u) => (
              <li key={u.username} className="rounded border border-slate-800 p-2">
                <p>
                  {u.username} · role={u.role} · status={u.status}
                </p>
                <p className="text-slate-500">created: {u.createdAt}</p>
                <div className="mt-2 flex flex-wrap gap-2">
                  <button
                    type="button"
                    disabled={updatingUser === u.username || u.status === "active"}
                    onClick={async () => {
                      setUpdatingUser(u.username);
                      try {
                        await updateAdminUserStatus({ username: u.username, status: "active" });
                        showToast("success", "user enabled");
                        await refreshUsers();
                        await refreshAuditLogs();
                      } catch (err) {
                        showToast("error", err instanceof Error ? err.message : "update status failed");
                      } finally {
                        setUpdatingUser("");
                      }
                    }}
                    className="rounded border border-emerald-700/60 px-2 py-1 text-[11px] text-emerald-200 disabled:opacity-50"
                  >
                    enable
                  </button>
                  <button
                    type="button"
                    disabled={updatingUser === u.username || u.status === "disabled"}
                    onClick={async () => {
                      setUpdatingUser(u.username);
                      try {
                        await updateAdminUserStatus({ username: u.username, status: "disabled" });
                        showToast("success", "user disabled");
                        await refreshUsers();
                        await refreshAuditLogs();
                      } catch (err) {
                        showToast("error", err instanceof Error ? err.message : "update status failed");
                      } finally {
                        setUpdatingUser("");
                      }
                    }}
                    className="rounded border border-amber-700/60 px-2 py-1 text-[11px] text-amber-200 disabled:opacity-50"
                  >
                    disable
                  </button>
                  <button
                    type="button"
                    onClick={() => setResetUsername(u.username)}
                    className="rounded border border-slate-700 px-2 py-1 text-[11px]"
                  >
                    set as reset target
                  </button>
                </div>
              </li>
            ))}
            {users.length === 0 ? <li className="text-slate-500">no users</li> : null}
          </ul>
          <div className="mt-4 rounded border border-slate-800 p-3">
            <p className="text-xs text-slate-400">Reset password</p>
            <div className="mt-2 space-y-2">
              <input
                value={resetUsername}
                onChange={(e) => setResetUsername(e.target.value)}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200"
                placeholder="target username"
              />
              <input
                value={resetPassword}
                onChange={(e) => setResetPassword(e.target.value)}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200"
                placeholder="new password"
                type="password"
              />
              <button
                type="button"
                disabled={resettingPassword}
                onClick={async () => {
                  if (!resetUsername.trim() || !resetPassword.trim()) {
                    showToast("error", "target username/new password required");
                    return;
                  }
                  setResettingPassword(true);
                  try {
                    await resetAdminUserPassword({
                      username: resetUsername.trim(),
                      newPassword: resetPassword.trim(),
                    });
                    showToast("success", "password reset done");
                    setResetPassword("");
                    await refreshAuditLogs();
                  } catch (err) {
                    showToast("error", err instanceof Error ? err.message : "reset password failed");
                  } finally {
                    setResettingPassword(false);
                  }
                }}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200 disabled:opacity-60"
              >
                {resettingPassword ? "resetting..." : "reset password"}
              </button>
            </div>
          </div>
          <div className="mt-4 rounded border border-slate-800 p-3">
            <div className="flex items-center justify-between">
              <p className="text-xs text-slate-300">User admin audit logs</p>
            </div>
            <div className="mt-2 grid gap-2 md:grid-cols-4">
              <input
                value={auditActorFilter}
                onChange={(e) => setAuditActorFilter(e.target.value)}
                className="rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-200"
                placeholder="filter actor"
              />
              <select
                value={auditActionFilter}
                onChange={(e) =>
                  setAuditActionFilter(
                    e.target.value as
                      | ""
                      | "admin.user.create"
                      | "admin.user.enable"
                      | "admin.user.disable"
                      | "admin.user.reset_password",
                  )
                }
                className="rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-200"
              >
                <option value="">all actions</option>
                <option value="admin.user.create">admin.user.create</option>
                <option value="admin.user.enable">admin.user.enable</option>
                <option value="admin.user.disable">admin.user.disable</option>
                <option value="admin.user.reset_password">admin.user.reset_password</option>
              </select>
              <input
                value={auditTargetFilter}
                onChange={(e) => setAuditTargetFilter(e.target.value)}
                className="rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-200"
                placeholder="filter target"
              />
              <button
                type="button"
                onClick={() => {
                  setAuditOffset(0);
                  refreshAuditLogs();
                }}
                className="rounded border border-slate-700 px-2 py-1 text-[11px] text-slate-200"
              >
                {loadingAuditLogs ? "loading..." : "apply filters"}
              </button>
            </div>
            <ul className="mt-2 space-y-2 text-xs text-slate-300">
              {auditLogs.map((log) => (
                <li key={log.id} className="rounded border border-slate-800 p-2">
                  <p>{log.createdAt} · {log.actor} · {log.action} · target={log.targetUsername}</p>
                </li>
              ))}
              {auditLogs.length === 0 ? <li className="text-slate-500">no audit logs</li> : null}
            </ul>
            <div className="mt-2 flex items-center justify-between text-[11px] text-slate-400">
              <p>offset={auditOffset}</p>
              <div className="flex gap-2">
                <button
                  type="button"
                  disabled={auditOffset <= 0 || loadingAuditLogs}
                  onClick={() => {
                    const next = Math.max(0, auditOffset - auditPageSize);
                    setAuditOffset(next);
                  }}
                  className="rounded border border-slate-700 px-2 py-1 disabled:opacity-50"
                >
                  prev
                </button>
                <button
                  type="button"
                  disabled={loadingAuditLogs || auditLogs.length < auditPageSize}
                  onClick={() => {
                    setAuditOffset(auditOffset + auditPageSize);
                  }}
                  className="rounded border border-slate-700 px-2 py-1 disabled:opacity-50"
                >
                  next
                </button>
                <button
                  type="button"
                  disabled={exportingAuditLogs}
                  onClick={() => {
                    handleExportAuditCsv();
                  }}
                  className="rounded border border-slate-700 px-2 py-1 disabled:opacity-50"
                >
                  {exportingAuditLogs ? "exporting..." : "export csv"}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    refreshAuditLogs();
                  }}
                  className="rounded border border-slate-700 px-2 py-1"
                >
                  refresh
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
