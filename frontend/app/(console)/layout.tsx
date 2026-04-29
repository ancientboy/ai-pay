import { ConsoleShell } from "@/components/console-shell";
import { getCurrentSession } from "@/lib/current-session";

export default async function ConsoleLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await getCurrentSession();
  return <ConsoleShell session={session}>{children}</ConsoleShell>;
}
