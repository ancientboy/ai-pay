import { ConsoleShell } from "@/components/console-shell";
import { HelpAssistantWidget } from "@/components/help-assistant-widget";

export default function ConsoleLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <ConsoleShell>
      {children}
      <HelpAssistantWidget />
    </ConsoleShell>
  );
}
