import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { LandingPage } from "@/components/landing-page";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";

export default async function Home() {
  const claims = await parseSessionToken((await cookies()).get(SESSION_COOKIE_NAME)?.value);
  if (claims?.sub) {
    redirect("/dashboard");
  }
  return <LandingPage />;
}
