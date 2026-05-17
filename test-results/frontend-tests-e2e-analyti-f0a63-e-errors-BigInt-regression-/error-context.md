# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: frontend\tests\e2e\analytics.spec.ts >> analytics >> changing the date range refetches without console errors (BigInt regression)
- Location: frontend\tests\e2e\analytics.spec.ts:32:3

# Error details

```
Error: page.goto: Protocol error (Page.navigate): Cannot navigate to invalid URL
Call log:
  - navigating to "/login", waiting until "load"

```

# Test source

```ts
  1  | import type { Page } from "@playwright/test";
  2  | import { expect, test as base } from "@playwright/test";
  3  | 
  4  | // Test users mirror config.yaml `bootstrap.*`. If you change those, mirror the
  5  | // changes here. Other roles can be added once we have UI-driven user creation
  6  | // in a fixture.
  7  | export const OWNER = {
  8  |   email: "owner@apotech.local",
  9  |   password: "test123",
  10 |   role: "OWNER" as const,
  11 | };
  12 | 
  13 | export type TestUser = typeof OWNER;
  14 | 
  15 | /**
  16 |  * Console errors we tolerate. Each entry is matched as a substring against
  17 |  * msg.text(). Use sparingly — every entry is a known upstream nuisance that
  18 |  * we've decided not to let the suite fail on.
  19 |  */
  20 | const ALLOWED_CONSOLE_NOISE = [
  21 |   // Chakra v3 dialog can race autofocus with focus-trap mount on the second
  22 |   // open of the same dialog instance. Visible to the user as nothing.
  23 |   "Your focus-trap needs to have at least one focusable element",
  24 |   // Vite dev-only HMR/source-map warnings.
  25 |   "404 (Not Found)", // favicon.ico
  26 |   "React Router Future Flag Warning",
  27 | ];
  28 | 
  29 | function isAllowed(text: string): boolean {
  30 |   return ALLOWED_CONSOLE_NOISE.some((s) => text.includes(s));
  31 | }
  32 | 
  33 | /**
  34 |  * Extended fixture that automatically fails any test which logs an unexpected
  35 |  * console error. This alone catches large classes of regressions (the BigInt
  36 |  * crash we hit today would fail every analytics test instantly).
  37 |  */
  38 | export const test = base.extend<{ page: Page }>({
  39 |   page: async ({ page }, use, testInfo) => {
  40 |     const errors: string[] = [];
  41 |     page.on("console", (msg) => {
  42 |       if (msg.type() === "error" && !isAllowed(msg.text())) {
  43 |         errors.push(msg.text());
  44 |       }
  45 |     });
  46 |     page.on("pageerror", (err) => {
  47 |       const text = err.message ?? String(err);
  48 |       if (!isAllowed(text)) errors.push(text);
  49 |     });
  50 | 
  51 |     await use(page);
  52 | 
  53 |     if (errors.length > 0 && testInfo.status === testInfo.expectedStatus) {
  54 |       throw new Error(
  55 |         `Unexpected console errors in ${testInfo.title}:\n  - ${errors.join("\n  - ")}`,
  56 |       );
  57 |     }
  58 |   },
  59 | });
  60 | 
  61 | export { expect };
  62 | 
  63 | /**
  64 |  * Log in via the /login form. Returns once we've landed on `/`. Throws if
  65 |  * credentials are rejected.
  66 |  */
  67 | export async function loginAs(page: Page, user: TestUser = OWNER): Promise<void> {
> 68 |   await page.goto("/login");
     |              ^ Error: page.goto: Protocol error (Page.navigate): Cannot navigate to invalid URL
  69 |   await page.getByRole("textbox", { name: "Email" }).fill(user.email);
  70 |   await page.getByRole("textbox", { name: "Password" }).fill(user.password);
  71 |   await page.getByRole("button", { name: "Sign in" }).click();
  72 |   await page.waitForURL(/\/(?!login)/);
  73 | }
  74 | 
  75 | /**
  76 |  * Clear auth tokens + persisted prefs from storage. Use in `beforeEach` to
  77 |  * guarantee a fresh state regardless of what the previous test left behind.
  78 |  */
  79 | export async function clearAuth(page: Page): Promise<void> {
  80 |   await page.goto("/");
  81 |   await page.evaluate(() => {
  82 |     localStorage.removeItem("apotech_access_token");
  83 |     localStorage.removeItem("apotech_refresh_token");
  84 |     localStorage.removeItem("apotech_branch_id");
  85 |   });
  86 | }
  87 | 
```