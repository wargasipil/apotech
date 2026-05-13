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
import { TOKEN_KEY } from "./transport";

type AuthState = {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
};

const AuthCtx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState<boolean>(
    () => localStorage.getItem(TOKEN_KEY) !== null,
  );

  useEffect(() => {
    if (localStorage.getItem(TOKEN_KEY) === null) {
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
        localStorage.removeItem(TOKEN_KEY);
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
    if (!res.token) throw new Error("no token in response");
    localStorage.setItem(TOKEN_KEY, res.token);
    setUser(res.user ?? null);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
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
