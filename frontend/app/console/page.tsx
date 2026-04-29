import { redirect } from "next/navigation";
import { getCurrentSession } from "@/lib/current-session";

export default async function ConsoleEntryPage() {
  const session = await getCurrentSession();
  if (!session) {
    redirect("/login?next=/console");
  }
  const role = (session.role || "operator").toLowerCase();
  if (role === "admin") {
    redirect("/admin-subscriptions");
  }
  if (role === "readonly") {
    redirect("/transactions");
  }
  redirect("/billing");
}
