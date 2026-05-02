"use client";

import { IntegrationGuide } from "@/components/integration-guide";
import { PublicHeader } from "@/components/public-header";

export default function PublicIntegrationDocPage() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <PublicHeader />
      <main className="mx-auto max-w-4xl px-6 py-12">
        <IntegrationGuide variant="public" />
      </main>
    </div>
  );
}
