import { cookies } from "next/headers";
import { ConsoleShell } from "@/components/console-shell";
import { HelpAssistantWidget } from "@/components/help-assistant-widget";
import { OnboardingTour } from "@/components/onboarding-tour";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import { redirect } from "next/navigation";

export default async function ConsoleLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const cookieStore = await cookies();
  const sessionToken = cookieStore.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(sessionToken);
  if (!claims) {
    redirect("/login");
  }
  return (
    <ConsoleShell session={claims}>
      {children}
      <OnboardingTour />
      <HelpAssistantWidget />
    </ConsoleShell>
  );
}
