import { test as setup } from "@playwright/test";

import { loginAs, OWNER } from "./_helpers";

// One-time auth: log in via the UI and persist the resulting localStorage
// (access + refresh tokens) to a JSON file. Every test project loads this
// file via `storageState`, so each spec starts pre-authenticated without
// calling the live Login RPC. Net effect: one login per `make test-browser`
// invocation instead of one per test (which would trip the backend's
// per-email rate limiter at ~5 attempts/minute).
export const STORAGE_STATE = "tests/e2e/.auth/owner.json";

setup("authenticate owner", async ({ page }) => {
  await loginAs(page, OWNER);
  await page.context().storageState({ path: STORAGE_STATE });
});
