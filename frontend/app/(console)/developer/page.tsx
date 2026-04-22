"use client";

import { useState } from "react";
import { useToast } from "@/components/toast-provider";

type ApiKeyItem = { id: string; name: string; key: string; createdAt: string };
type WebhookItem = { id: string; url: string; event: string; createdAt: string };

const API_KEYS_STORAGE = "ai-pay.apiKeys";
const WEBHOOKS_STORAGE = "ai-pay.webhooks";

export default function DeveloperPage() {
  const { showToast } = useToast();
  const [apiKeyName, setApiKeyName] = useState("default");
  const [webhookURL, setWebhookURL] = useState("");
  const [webhookEvent, setWebhookEvent] = useState("payment.settled");
  const [apiKeys, setApiKeys] = useState<ApiKeyItem[]>(() => {
    if (typeof window === "undefined") {
      return [];
    }
    const keys = window.localStorage.getItem(API_KEYS_STORAGE);
    return keys ? (JSON.parse(keys) as ApiKeyItem[]) : [];
  });
  const [webhooks, setWebhooks] = useState<WebhookItem[]>(() => {
    if (typeof window === "undefined") {
      return [];
    }
    const hooks = window.localStorage.getItem(WEBHOOKS_STORAGE);
    return hooks ? (JSON.parse(hooks) as WebhookItem[]) : [];
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Developer Center</h2>
        <p className="mt-1 text-sm text-slate-400">
          Manage API keys and webhooks for agent integrations.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">API Keys</h3>
          <div className="mt-3 flex gap-2">
            <input
              value={apiKeyName}
              onChange={(e) => setApiKeyName(e.target.value)}
              className="flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder="Key name"
            />
            <button
              onClick={() => {
                if (!apiKeyName.trim()) {
                  showToast("error", "API key name required");
                  return;
                }
                const item: ApiKeyItem = {
                  id: `key_${Date.now()}`,
                  name: apiKeyName.trim(),
                  key: `ak_live_${Math.random().toString(36).slice(2, 14)}`,
                  createdAt: new Date().toISOString(),
                };
                const next = [item, ...apiKeys];
                setApiKeys(next);
                window.localStorage.setItem(API_KEYS_STORAGE, JSON.stringify(next));
                showToast("success", "API key created");
              }}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
            >
              Create
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {apiKeys.map((item) => (
              <li key={item.id} className="rounded border border-slate-800 p-2">
                <p>{item.name}</p>
                <p className="font-mono text-slate-400">{item.key}</p>
              </li>
            ))}
            {apiKeys.length === 0 ? <li className="text-slate-500">No API keys.</li> : null}
          </ul>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">Webhooks</h3>
          <div className="mt-3 space-y-2">
            <input
              value={webhookURL}
              onChange={(e) => setWebhookURL(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder="https://your-app.com/webhook"
            />
            <input
              value={webhookEvent}
              onChange={(e) => setWebhookEvent(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder="event name"
            />
            <button
              onClick={() => {
                if (!webhookURL.trim()) {
                  showToast("error", "Webhook URL required");
                  return;
                }
                const item: WebhookItem = {
                  id: `wh_${Date.now()}`,
                  url: webhookURL.trim(),
                  event: webhookEvent.trim() || "payment.settled",
                  createdAt: new Date().toISOString(),
                };
                const next = [item, ...webhooks];
                setWebhooks(next);
                window.localStorage.setItem(WEBHOOKS_STORAGE, JSON.stringify(next));
                showToast("success", "Webhook created");
              }}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
            >
              Add Webhook
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {webhooks.map((item) => (
              <li key={item.id} className="rounded border border-slate-800 p-2">
                <p className="font-mono text-slate-400">{item.url}</p>
                <p>{item.event}</p>
              </li>
            ))}
            {webhooks.length === 0 ? <li className="text-slate-500">No webhooks.</li> : null}
          </ul>
        </div>
      </div>
    </section>
  );
}
