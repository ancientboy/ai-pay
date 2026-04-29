import { ConsoleShell } from "@/components/console-shell";
import { FirstRunGuideModal } from "@/components/first-run-guide-modal";
import { getCurrentSession } from "@/lib/current-session";

export default async function ConsoleLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await getCurrentSession();
  return (
    <ConsoleShell session={session}>
      <FirstRunGuideModal />
      {children}
    </ConsoleShell>
  );
}
