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

export type PlatformUser = {
  id: number;
  full_name: string;
  avatar_url?: string;
  roles: string[];
};

type PlatformSessionState = {
  user?: PlatformUser;
  roles?: string[];
  isAuthenticated: boolean;
  isReady: boolean;
  isAuthorized: boolean;
  refresh: () => Promise<void>;
  logout: () => void;
};

const PlatformSessionContext = createContext<PlatformSessionState | null>(null);

const PLATFORM_ROLES = ["admin"];

export function PlatformSessionProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [user, setUser] = useState<PlatformUser | undefined>(undefined);
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
    const userInfo = await apiGet<PlatformUser>("/users/me");
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
    () => roles.some((role) => PLATFORM_ROLES.includes(role)),
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
    <PlatformSessionContext.Provider value={value}>
      {children}
    </PlatformSessionContext.Provider>
  );
}

export function usePlatformSession() {
  return useContext(PlatformSessionContext);
}
