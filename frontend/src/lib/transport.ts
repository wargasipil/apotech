import { Code, ConnectError, createPromiseClient } from "@connectrpc/connect";
import type { Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { AuthService } from "../gen/user_iface/v1/auth_connect";

export const ACCESS_KEY = "apotech_access_token";
export const REFRESH_KEY = "apotech_refresh_token";
export const WAREHOUSE_KEY = "apotech_warehouse_id";

// Dedicated transport without the auth interceptor: used to call Refresh so
// we don't recurse on 401.
const noAuthTransport = createConnectTransport({ baseUrl: "/api" });
const refreshClient = createPromiseClient(AuthService, noAuthTransport);

// Singleton in-flight refresh promise so concurrent requests on an expired
// access token trigger only ONE Refresh call.
let refreshing: Promise<boolean> | null = null;

function clearTokens() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

async function doRefresh(): Promise<boolean> {
  const rt = localStorage.getItem(REFRESH_KEY);
  if (!rt) return false;
  try {
    const res = await refreshClient.refresh({ refreshToken: rt });
    if (!res.accessToken || !res.refreshToken) {
      clearTokens();
      return false;
    }
    localStorage.setItem(ACCESS_KEY, res.accessToken);
    localStorage.setItem(REFRESH_KEY, res.refreshToken);
    return true;
  } catch {
    clearTokens();
    return false;
  }
}

async function tryRefresh(): Promise<boolean> {
  if (refreshing) return refreshing;
  refreshing = doRefresh().finally(() => {
    refreshing = null;
  });
  return refreshing;
}

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = localStorage.getItem(ACCESS_KEY);
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  const warehouseId = localStorage.getItem(WAREHOUSE_KEY);
  if (warehouseId) {
    req.header.set("X-Warehouse-Id", warehouseId);
  }
  try {
    return await next(req);
  } catch (err) {
    // Don't try to refresh the Refresh call itself, or unauthenticated
    // requests that never had a token to begin with.
    const isRefreshCall = req.url.endsWith("/AuthService/Refresh");
    const isUnauthenticated =
      err instanceof ConnectError && err.code === Code.Unauthenticated;

    if (!isUnauthenticated || !token || isRefreshCall) throw err;

    const ok = await tryRefresh();
    if (!ok) throw err;

    const fresh = localStorage.getItem(ACCESS_KEY);
    if (fresh) {
      req.header.set("Authorization", `Bearer ${fresh}`);
    }
    return next(req);
  }
};

export const transport = createConnectTransport({
  baseUrl: "/api",
  interceptors: [authInterceptor],
});
