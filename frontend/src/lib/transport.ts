import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";

export const TOKEN_KEY = "apotech_token";

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};

export const transport = createConnectTransport({
  baseUrl: "/api",
  interceptors: [authInterceptor],
});
