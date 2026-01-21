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
};

type MerchantStatus = {
  is_open: boolean;
  auto_close_at?: string;
  message?: string;
};

type MerchantSessionState = {
  merchant?: MerchantInfo;
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
  const [status, setStatus] = useState<MerchantStatus | undefined>(undefined);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isReady, setIsReady] = useState(false);
  const [wsConnected, setWsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const isConnectingRef = useRef(false);

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
    const [merchantInfo, merchantStatus] = await Promise.all([
      apiGet<MerchantInfo>("/merchants/me"),
      apiGet<MerchantStatus>("/merchants/me/status"),
    ]);
    setMerchant(merchantInfo);
    setStatus(merchantStatus);
    setIsReady(true);
  }, []);

  const closeWebSocket = useCallback(() => {
    if (wsRef.current) {
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

    isConnectingRef.current = true;
    const socket = new WebSocket(wsUrl);
    wsRef.current = socket;

    socket.onopen = () => {
      setWsConnected(true);
      isConnectingRef.current = false;
    };

    socket.onclose = () => {
      setWsConnected(false);
      isConnectingRef.current = false;
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
        window.dispatchEvent(
          new CustomEvent("merchant-realtime", { detail: event.data })
        );
      }
    };
  }, []);

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
    setStatus(undefined);
    setIsReady(true);
  }, [closeWebSocket]);

  useEffect(() => {
    refresh().catch(() => {
      setIsAuthenticated(false);
      setIsReady(true);
    });
  }, [refresh]);

  useEffect(() => {
    if (!isAuthenticated) {
      closeWebSocket();
      return;
    }
    if (isOpen) {
      connectWebSocket();
    } else {
      closeWebSocket();
    }
  }, [isAuthenticated, isOpen, connectWebSocket, closeWebSocket]);

  useEffect(() => {
    return () => {
      closeWebSocket();
    };
  }, [closeWebSocket]);

  const value = useMemo(
    () => ({
      merchant,
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

