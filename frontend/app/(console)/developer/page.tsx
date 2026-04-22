"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  createDeveloperApiKey,
  createDeveloperWebhook,
  listDeveloperApiKeys,
  listDeveloperWebhooks,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";

type ApiKeyItem = { id: string; name: string; key: string; createdAt: string };
type WebhookItem = { id: string; url: string; event: string; createdAt: string };

export default function DeveloperPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [apiKeyName, setApiKeyName] = useState("default");
  const [webhookURL, setWebhookURL] = useState("");
  const [webhookEvent, setWebhookEvent] = useState("payment.settled");

  const apiKeysQuery = useQuery({
    queryKey: ["developer", "apiKeys"],
    queryFn: listDeveloperApiKeys,
  });

  const webhooksQuery = useQuery({
    queryKey: ["developer", "webhooks"],
    queryFn: listDeveloperWebhooks,
  });

  const createApiKeyMutation = useMutation({
    mutationFn: (name: string) => createDeveloperApiKey(name),
    onSuccess: (item) => {
      setApiKeyName("default");
      showToast("success", t("developer.keyCreated"));
      queryClient.setQueryData<ApiKeyItem[]>(["developer", "apiKeys"], (prev) => [
        item,
        ...(prev ?? []),
      ]);
      queryClient.invalidateQueries({ queryKey: ["developer", "apiKeys"] });
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });

  const createWebhookMutation = useMutation({
    mutationFn: (input: { url: string; event: string }) =>
      createDeveloperWebhook({ url: input.url, event: input.event }),
    onSuccess: (item) => {
      setWebhookURL("");
      setWebhookEvent("payment.settled");
      showToast("success", t("developer.webhookCreated"));
      queryClient.setQueryData<WebhookItem[]>(["developer", "webhooks"], (prev) => [
        item,
        ...(prev ?? []),
      ]);
      queryClient.invalidateQueries({ queryKey: ["developer", "webhooks"] });
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("developer.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("developer.subtitle")}</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("developer.apiKeys")}</h3>
          <div className="mt-3 flex gap-2">
            <input
              value={apiKeyName}
              onChange={(e) => setApiKeyName(e.target.value)}
              className="flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("developer.keyName")}
            />
            <button
              onClick={() => {
                if (!apiKeyName.trim()) {
                  showToast("error", t("developer.keyNameRequired"));
                  return;
                }
                createApiKeyMutation.mutate(apiKeyName.trim());
              }}
              disabled={createApiKeyMutation.isPending}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
            >
              {t("common.create")}
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {(apiKeysQuery.data ?? []).map((item) => (
              <li key={item.id} className="rounded border border-slate-800 p-2">
                <p>{item.name}</p>
                <p className="font-mono text-slate-400">{item.key}</p>
              </li>
            ))}
            {(apiKeysQuery.data ?? []).length === 0 ? (
              <li className="text-slate-500">{t("developer.noKeys")}</li>
            ) : null}
          </ul>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("developer.webhooks")}</h3>
          <div className="mt-3 space-y-2">
            <input
              value={webhookURL}
              onChange={(e) => setWebhookURL(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("developer.webhookUrl")}
            />
            <input
              value={webhookEvent}
              onChange={(e) => setWebhookEvent(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("developer.webhookEvent")}
            />
            <button
              onClick={() => {
                if (!webhookURL.trim()) {
                  showToast("error", t("developer.webhookUrlRequired"));
                  return;
                }
                createWebhookMutation.mutate({
                  url: webhookURL.trim(),
                  event: webhookEvent.trim() || "payment.settled",
                });
              }}
              disabled={createWebhookMutation.isPending}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
            >
              {t("developer.addWebhook")}
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {(webhooksQuery.data ?? []).map((item) => (
              <li key={item.id} className="rounded border border-slate-800 p-2">
                <p className="font-mono text-slate-400">{item.url}</p>
                <p>{item.event}</p>
              </li>
            ))}
            {(webhooksQuery.data ?? []).length === 0 ? (
              <li className="text-slate-500">{t("developer.noWebhooks")}</li>
            ) : null}
          </ul>
        </div>
      </div>
    </section>
  );
}
