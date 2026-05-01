import { expect, test } from "@playwright/test";

test("commercial auth flow: register, login, and role guard", async ({ page, request }) => {
  const seed = Date.now();
  const username = `pw_user_${seed}`;
  const password = "Passw0rd!2";

  const registerResp = await request.post("/api/auth/register", {
    data: { username, password },
  });
  expect(registerResp.ok()).toBeTruthy();
  const registerPayload = (await registerResp.json()) as { code?: string };
  expect(registerPayload.code).toBe("0");

  await page.goto("/login");
  await page.locator('input[autocomplete="username"]').fill(username);
  await page.locator('input[autocomplete="current-password"]').fill(password);
  await page.getByRole("button", { name: /登录|Login/i }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto("/developer");
  await expect(page).toHaveURL(/\/dashboard$/);

  const adminLoginResp = await request.post("/api/auth/login", {
    data: { username: "admin", password: "admin123" },
  });
  expect(adminLoginResp.ok()).toBeTruthy();
  const setCookie = adminLoginResp.headers()["set-cookie"] ?? "";
  const cookie = setCookie.split(";")[0] ?? "";

  const logsResp = await request.get("/api/auth/admin/users?view=audit", {
    headers: cookie ? { cookie } : undefined,
  });
  expect(logsResp.ok()).toBeTruthy();
  const logsPayload = (await logsResp.json()) as {
    code?: string;
    data?: { logs?: Array<{ action?: string; targetUsername?: string }> };
  };
  expect(logsPayload.code).toBe("0");
  const hasCreateLog = (logsPayload.data?.logs ?? []).some(
    (item) =>
      item.action === "admin.user.set_role" || item.action === "admin.user.set_plan",
  );
  expect(hasCreateLog).toBeTruthy();
});
