import { clearAuth, expect, loginAs, OWNER, test } from "./_helpers";

// Auth tests test the auth flow itself, so they must start with no stored
// tokens. Override storageState to an empty origin.
test.use({ storageState: { cookies: [], origins: [] } });

test.describe("auth", () => {
  test.beforeEach(async ({ page }) => {
    await clearAuth(page);
  });

  test("logs in with valid credentials and lands on dashboard", async ({ page }) => {
    await loginAs(page);
    await expect(page).toHaveURL("/");
    // Authenticated layout renders the sidebar's Sign-out button. More
    // stable than asserting the email (which appears in multiple spots).
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
  });

  test("rejects wrong password without leaving /login", async ({ page }) => {
    await page.goto("/login");
    await page.getByRole("textbox", { name: "Email" }).fill(OWNER.email);
    await page.getByRole("textbox", { name: "Password" }).fill("definitely-wrong");
    await page.getByRole("button", { name: "Sign in" }).click();
    // Login.tsx opts out of the global toast (meta.silentError) and renders an
    // inline error. We just assert we did NOT navigate away from /login.
    await page.waitForTimeout(500); // small grace for the response
    await expect(page).toHaveURL(/\/login$/);
  });

  test("logout clears tokens and redirects to /login", async ({ browser }) => {
    // Use a fresh context with the shared storageState so we don't burn a
    // Login attempt against the per-email rate limiter for this test.
    const ctx = await browser.newContext({ storageState: "tests/e2e/.auth/owner.json" });
    const page = await ctx.newPage();
    await page.goto("/");
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
    await page.getByRole("button", { name: "Sign out" }).click();
    await page.waitForURL(/\/login$/);
    const tokens = await page.evaluate(() => ({
      access: localStorage.getItem("apotech_access_token"),
      refresh: localStorage.getItem("apotech_refresh_token"),
    }));
    expect(tokens.access).toBeNull();
    expect(tokens.refresh).toBeNull();
    await ctx.close();
  });
});
