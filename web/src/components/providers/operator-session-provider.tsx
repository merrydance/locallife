"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { apiGet, getAuthToken } from "@/lib/api";

export type OperatorUser = {
  id: number;
  full_name: string;
  avatar_url?: string;
  roles: string[];
};

type OperatorSessionState = {
  user?: OperatorUser;
  roles?: string[];
  isAuthenticated: boolean;
  isReady: boolean;
  isAuthorized: boolean;
  refresh: () => Promise<void>;
  logout: () => void;
};

const OperatorSessionContext = createContext<OperatorSessionState | null>(null);

const OPERATOR_ROLES = ["operator"];

export function OperatorSessionProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [user, setUser] = useState<OperatorUser | undefined>(undefined);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isReady, setIsReady] = useState(false);

  const refresh = useCallback(async () => {
    const token = getAuthToken();
    if (!token) {
      setIsAuthenticated(false);
      setUser(undefined);
      setIsReady(true);
      return;
    }

    setIsAuthenticated(true);
    const userInfo = await apiGet<OperatorUser>("/users/me");
    setUser(userInfo);
    setIsReady(true);
  }, []);

  const logout = useCallback(() => {
    if (typeof window !== "undefined") {
      window.localStorage.removeItem("access_token");
      window.localStorage.removeItem("refresh_token");
    }
    setIsAuthenticated(false);
    setUser(undefined);
    setIsReady(true);
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => {
      refresh().catch(() => {
        setIsAuthenticated(false);
        setUser(undefined);
        setIsReady(true);
      });
    }, 0);
    return () => clearTimeout(timer);
  }, [refresh]);

  const roles = useMemo(() => user?.roles ?? [], [user?.roles]);
  const isAuthorized = useMemo(
    () => roles.some((role) => OPERATOR_ROLES.includes(role)),
    [roles]
  );

  const value = useMemo(
    () => ({
      user,
      roles,
      isAuthenticated,
      isReady,
      isAuthorized,
      refresh,
      logout,
    }),
    [user, roles, isAuthenticated, isReady, isAuthorized, refresh, logout]
  );

  return (
    <OperatorSessionContext.Provider value={value}>
      {children}
    </OperatorSessionContext.Provider>
  );
}

export function useOperatorSession() {
  return useContext(OperatorSessionContext);
}
