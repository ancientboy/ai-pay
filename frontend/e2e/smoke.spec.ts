import { expect, test } from "@playwright/test";

test("login page is reachable", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("AI Pay");
});

test("recharge page supports week2 account flows", async ({ page, request }) => {
  const seed = Date.now();
  const didA = `did:gusd:agent:e2e_week2_a_${seed}`;
  const didB = `did:gusd:agent:e2e_week2_b_${seed}`;
  const didPubKey = Buffer.alloc(32, 7).toString("base64");

  const registerA = await request.post("/api/backend/agent/did/register", {
    data: { agentDid: didA, didPubKey },
  });
  expect(registerA.ok()).toBeTruthy();
  const registerB = await request.post("/api/backend/agent/did/register", {
    data: { agentDid: didB, didPubKey },
  });
  expect(registerB.ok()).toBeTruthy();

  const createA = await request.post("/api/backend/account/create", {
    data: { agentDid: didA },
  });
  const createAJson = await createA.json();
  const fromVA = createAJson.data.VAAccountID as string;
  const createB = await request.post("/api/backend/account/create", {
    data: { agentDid: didB },
  });
  const createBJson = await createB.json();
  const toVA = createBJson.data.VAAccountID as string;

  const recharge = await request.post("/api/backend/fund/recharge", {
    headers: { "Idempotency-Key": `e2e-rch-${seed}` },
    data: { vaAccountId: fromVA, amount: "20" },
  });
  expect(recharge.ok()).toBeTruthy();

  await page.goto("/login");
  await page.locator('input[autocomplete="username"]').fill("admin");
  await page.locator('input[autocomplete="current-password"]').fill("admin123");
  await page.locator('input[autocomplete="current-password"]').press("Enter");
  await page.waitForURL("**/dashboard");

  await page.goto("/recharge");

  await page.getByPlaceholder("查询账户").fill(fromVA);
  await page.getByRole("button", { name: "查询利息" }).click();
  await expect(page.getByText(`查询账户: ${fromVA}`)).toBeVisible();

  await page.getByPlaceholder("查询账户").nth(1).fill(fromVA);
  await page.getByPlaceholder("触发阈值").fill("5");
  await page.getByPlaceholder("补足目标").fill("30");
  await page.getByRole("button", { name: "保存自动充值配置" }).click();
  await expect(page.getByText("自动充值配置已保存")).toBeVisible();

  await page.getByPlaceholder("转出账户").fill(fromVA);
  await page.getByPlaceholder("转入账户").fill(toVA);
  await page.getByPlaceholder("转账金额").fill("3");
  await page.getByRole("button", { name: "提交转账" }).click();
  await expect(page.getByText("VA 转账成功")).toBeVisible();

  await page.locator("select").first().selectOption("SETTLED");
  await page.getByRole("button", { name: /(应用筛选|Apply Filters)/ }).click();
  await expect(page.getByText(/(Transfer Page|转账分页):\s*1/)).toBeVisible();

  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: /(导出 CSV|Export CSV)/ }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toContain("va-transfer-");
});

