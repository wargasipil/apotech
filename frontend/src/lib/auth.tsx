import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { User } from "../gen/user_iface/v1/users_pb";
import { authClient } from "./clients";
import { ACCESS_KEY, REFRESH_KEY } from "./transport";

type AuthState = {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthCtx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState<boolean>(
    () => localStorage.getItem(ACCESS_KEY) !== null,
  );

  useEffect(() => {
    if (localStorage.getItem(ACCESS_KEY) === null) {
      setLoading(false);
      return;
    }
    let cancelled = false;
    authClient
      .me({})
      .then((res) => {
        if (cancelled) return;
        setUser(res.user ?? null);
      })
      .catch(() => {
        if (cancelled) return;
        // Transport already cleared tokens on refresh failure.
        setUser(null);
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const res = await authClient.login({ email, password });
    if (!res.accessToken || !res.refreshToken) {
      throw new Error("no token in response");
    }
    localStorage.setItem(ACCESS_KEY, res.accessToken);
    localStorage.setItem(REFRESH_KEY, res.refreshToken);
    setUser(res.user ?? null);
  }, []);

  const logout = useCallback(async () => {
    const refreshToken = localStorage.getItem(REFRESH_KEY) ?? "";
    if (refreshToken) {
      try {
        await authClient.logout({ refreshToken });
      } catch {
        // best-effort
      }
    }
    localStorage.removeItem(ACCESS_KEY);
    localStorage.removeItem(REFRESH_KEY);
    setUser(null);
  }, []);

  const value = useMemo<AuthState>(
    () => ({ user, loading, login, logout }),
    [user, loading, login, logout],
  );

  return <AuthCtx.Provider value={value}>{children}</AuthCtx.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthCtx);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}
