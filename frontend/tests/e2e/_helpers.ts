import type { Page } from "@playwright/test";
import { expect, test as base } from "@playwright/test";

// Test users mirror config.yaml `bootstrap.*`. If you change those, mirror the
// changes here. Other roles can be added once we have UI-driven user creation
// in a fixture.
export const OWNER = {
  email: "owner@apotech.local",
  password: "test123",
  role: "OWNER" as const,
};

export type TestUser = typeof OWNER;

/**
 * Console errors we tolerate. Each entry is matched as a substring against
 * msg.text(). Use sparingly — every entry is a known upstream nuisance that
 * we've decided not to let the suite fail on.
 */
const ALLOWED_CONSOLE_NOISE = [
  // Chakra v3 dialog can race autofocus with focus-trap mount on the second
  // open of the same dialog instance. Visible to the user as nothing.
  "Your focus-trap needs to have at least one focusable element",
  // Vite dev-only HMR/source-map warnings.
  "404 (Not Found)", // favicon.ico
  "React Router Future Flag Warning",
  // Connect RPC errors are surfaced via toast; the browser also logs them at
  // the resource-load level. Tests for *expected* failures (wrong password,
  // FailedPrecondition) would otherwise trip the fixture on this noise.
  "401 (Unauthorized)",
  "412 (Precondition Failed)",
  "429 (Too Many Requests)",
];

function isAllowed(text: string): boolean {
  return ALLOWED_CONSOLE_NOISE.some((s) => text.includes(s));
}

/**
 * Extended fixture that automatically fails any test which logs an unexpected
 * console error. This alone catches large classes of regressions (the BigInt
 * crash we hit today would fail every analytics test instantly).
 */
export const test = base.extend<{ page: Page }>({
  page: async ({ page }, use, testInfo) => {
    const errors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" && !isAllowed(msg.text())) {
        errors.push(msg.text());
      }
    });
    page.on("pageerror", (err) => {
      const text = err.message ?? String(err);
      if (!isAllowed(text)) errors.push(text);
    });

    await use(page);

    if (errors.length > 0 && testInfo.status === testInfo.expectedStatus) {
      throw new Error(
        `Unexpected console errors in ${testInfo.title}:\n  - ${errors.join("\n  - ")}`,
      );
    }
  },
});

export { expect };

/**
 * Log in via the /login form. Returns once we've landed on `/`. Throws if
 * credentials are rejected.
 */
export async function loginAs(page: Page, user: TestUser = OWNER): Promise<void> {
  await page.goto("/login");
  await page.getByRole("textbox", { name: "Email" }).fill(user.email);
  await page.getByRole("textbox", { name: "Password" }).fill(user.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  // Use a path-based matcher so the `//` in the protocol doesn't accidentally
  // match a regex like `/(?!login)/`.
  await page.waitForURL((url) => !url.pathname.endsWith("/login"));
}

/**
 * Clear auth tokens + persisted prefs from storage. Use in `beforeEach` to
 * guarantee a fresh state regardless of what the previous test left behind.
 */
export async function clearAuth(page: Page): Promise<void> {
  await page.goto("/");
  await page.evaluate(() => {
    localStorage.removeItem("apotech_access_token");
    localStorage.removeItem("apotech_refresh_token");
    localStorage.removeItem("apotech_branch_id");
  });
}
