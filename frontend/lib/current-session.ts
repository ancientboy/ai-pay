import { cookies } from "next/headers";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";

export async function getCurrentSession() {
  const jar = await cookies();
  const token = jar.get(SESSION_COOKIE_NAME)?.value;
  return parseSessionToken(token);
}
