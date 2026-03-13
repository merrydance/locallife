"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { apiGet, apiPatch, getAuthToken } from "@/lib/api";

type MerchantInfo = {
  id: number;
  name: string;
  is_open?: boolean;
  group_id?: number;
  brand_id?: number;
};

type User = {
  id: number;
  full_name: string;
  avatar_url?: string;
  roles: string[];
};

type MerchantStatus = {
  is_open: boolean;
  auto_close_at?: string;
  message?: string;
};

type MerchantSessionState = {
  merchant?: MerchantInfo;
  user?: User;
  roles?: string[];
  status?: MerchantStatus;
  isAuthenticated: boolean;
  isReady: boolean;
  isOpen: boolean;
  wsConnected: boolean;
  refresh: () => Promise<void>;
  setOpen: (nextOpen: boolean) => Promise<void>;
  logout: () => void;
};

type RealtimeMessage = {
  type: string;
  channel?: string;
  data?: unknown;
  timestamp?: string;
  sequence?: number;
};

const MerchantSessionContext = createContext<MerchantSessionState | null>(null);

function buildWebSocketUrl() {
  const token = getAuthToken();
  if (!token) return null;

  const base = (process.env.NEXT_PUBLIC_API_BASE?.trim() || "/v1").replace(
    /\/$/,
    ""
  );
  const httpBase = base.startsWith("http")
    ? base
    : typeof window !== "undefined"
    ? `${window.location.origin}${base}`
    : base;

  const wsBase = httpBase
    .replace("https://", "wss://")
    .replace("http://", "ws://");

  return `${wsBase}/ws?token=${encodeURIComponent(token)}`;
}

export function MerchantSessionProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [merchant, setMerchant] = useState<MerchantInfo | undefined>(undefined);
  const [user, setUser] = useState<User | undefined>(undefined);
  const [status, setStatus] = useState<MerchantStatus | undefined>(undefined);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isReady, setIsReady] = useState(false);
  const [wsConnected, setWsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const isConnectingRef = useRef(false);
  const intentionalCloseRef = useRef(false);
  const reconnectTimerRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectDelayRef = useRef(1000);
  const connectWebSocketRef = useRef<() => void>(() => {});

  const isOpen = status?.is_open ?? merchant?.is_open ?? false;

  const refresh = useCallback(async () => {
    const token = getAuthToken();
    if (!token) {
      setIsAuthenticated(false);
      setMerchant(undefined);
      setStatus(undefined);
      setIsReady(true);
      return;
    }

    setIsAuthenticated(true);
    const [merchantInfo, merchantStatus, userInfo] = await Promise.all([
      apiGet<MerchantInfo>("/merchants/me"),
      apiGet<MerchantStatus>("/merchants/me/status"),
      apiGet<User>("/users/me"),
    ]);
    setMerchant(merchantInfo);
    setStatus(merchantStatus);
    setUser(userInfo);
    setIsReady(true);
  }, []);

  const closeWebSocket = useCallback(() => {
    // Cancel any pending auto-reconnect timer first
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    if (wsRef.current) {
      // Mark as intentional so onclose does not schedule a reconnect
      intentionalCloseRef.current = true;
      wsRef.current.close();
      wsRef.current = null;
    }
    setWsConnected(false);
    isConnectingRef.current = false;
  }, []);

  const connectWebSocket = useCallback(() => {
    if (wsRef.current || isConnectingRef.current) return;
    const wsUrl = buildWebSocketUrl();
    if (!wsUrl) return;

    intentionalCloseRef.current = false;
    isConnectingRef.current = true;
    const socket = new WebSocket(wsUrl);
    wsRef.current = socket;

    socket.onopen = () => {
      console.log("WebSocket Connected");
      setWsConnected(true);
      isConnectingRef.current = false;
      reconnectDelayRef.current = 1000; // Reset delay on success
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    socket.onclose = () => {
      console.log("WebSocket Disconnected");
      setWsConnected(false);
      wsRef.current = null;
      isConnectingRef.current = false;

      // Only auto-reconnect for unexpected disconnections.
      // Intentional closes (打烊/logout) set intentionalCloseRef=true to suppress this.
      if (!intentionalCloseRef.current) {
        if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = setTimeout(() => {
          console.log(`Attempting reconnect in ${reconnectDelayRef.current}ms...`);
          reconnectDelayRef.current = Math.min(reconnectDelayRef.current * 1.5, 30000);
          connectWebSocketRef.current();
        }, reconnectDelayRef.current);
      }
      intentionalCloseRef.current = false;
    };

    socket.onerror = () => {
      setWsConnected(false);
      isConnectingRef.current = false;
    };

    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as RealtimeMessage;
        window.dispatchEvent(
          new CustomEvent("merchant-realtime", { detail: message })
        );
      } catch {
        // Handle non-JSON or raw data if needed
      }
    };
  }, []);

  useEffect(() => {
    connectWebSocketRef.current = connectWebSocket;
  }, [connectWebSocket]);

  const setOpen = useCallback(
    async (nextOpen: boolean) => {
      const updated = await apiPatch<MerchantStatus>(
        "/merchants/me/status",
        { is_open: nextOpen }
      );
      setStatus(updated);
    },
    []
  );

  const logout = useCallback(() => {
    if (typeof window !== "undefined") {
      window.localStorage.removeItem("access_token");
      window.localStorage.removeItem("refresh_token");
    }
    closeWebSocket();
    setIsAuthenticated(false);
    setMerchant(undefined);
    setUser(undefined);
    setStatus(undefined);
    setIsReady(true);
  }, [closeWebSocket]);

  useEffect(() => {
    const timer = setTimeout(() => {
      refresh().catch(() => {
        setIsAuthenticated(false);
        setIsReady(true);
      });
    }, 0);
    return () => clearTimeout(timer);
  }, [refresh]);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (!isAuthenticated) {
        closeWebSocket();
        return;
      }
      if (isOpen) {
        connectWebSocket();
      } else {
        closeWebSocket();
      }
    }, 0);
    return () => clearTimeout(timer);
  }, [isAuthenticated, isOpen, connectWebSocket, closeWebSocket]);

  useEffect(() => {
    const handler = (event: Event) => {
      const customEvent = event as CustomEvent;
      const detail = customEvent.detail as RealtimeMessage;
      
      if (detail?.type === "merchant_status_update" && detail.data) {
        setStatus(detail.data as MerchantStatus);
      }
    };

    window.addEventListener("merchant-realtime", handler);
    return () => {
      window.removeEventListener("merchant-realtime", handler);
      closeWebSocket();
    };
  }, [closeWebSocket]);

  const value = useMemo(
    () => ({
      merchant,
      user,
      roles: user?.roles,
      status,
      isAuthenticated,
      isReady,
      isOpen,
      wsConnected,
      refresh,
      setOpen,
      logout,
    }),
    [
      merchant,
      user,
      status,
      isAuthenticated,
      isReady,
      isOpen,
      wsConnected,
      refresh,
      setOpen,
      logout,
    ]
  );

  return (
    <MerchantSessionContext.Provider value={value}>
      {children}
    </MerchantSessionContext.Provider>
  );
}

export function useMerchantSession() {
  return useContext(MerchantSessionContext);
}

