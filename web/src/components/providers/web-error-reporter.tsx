"use client";

import { useEffect } from "react";
import { API_BASE, getAuthToken } from "@/lib/api";

type WebErrorPayload = {
  source: "web";
  kind: "error" | "unhandledrejection";
  message: string;
  stack?: string;
  page: string;
  timestamp: number;
  userAgent: string;
};

const REPORT_INTERVAL_MS = 5000;
const recentReports = new Map<string, number>();

function shouldReport(signature: string): boolean {
  const now = Date.now();
  const last = recentReports.get(signature) || 0;
  if (now-last < REPORT_INTERVAL_MS) {
    return false;
  }
  recentReports.set(signature, now);
  return true;
}

function sendErrorLog(payload: WebErrorPayload) {
  const token = getAuthToken();
  if (!token) {
    return;
  }

  const url = `${API_BASE}/logs/error`;

  fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Response-Envelope": "1",
      Authorization: `Bearer ${token}`,
      "X-Client-Platform": "web",
    },
    body: JSON.stringify(payload),
    keepalive: true,
    cache: "no-store",
  }).catch(() => {
    // 静默失败，不能影响主流程
  });
}

export function WebErrorReporter() {
  useEffect(() => {
    if (process.env.NODE_ENV !== "production") {
      return;
    }

    const onError = (event: ErrorEvent) => {
      const message = event.message || "unknown error";
      const stack = event.error instanceof Error ? event.error.stack : undefined;
      const signature = `error:${message}:${stack || ""}`;
      if (!shouldReport(signature)) {
        return;
      }

      sendErrorLog({
        source: "web",
        kind: "error",
        message,
        stack,
        page: typeof window !== "undefined" ? window.location.href : "unknown",
        timestamp: Date.now(),
        userAgent: typeof navigator !== "undefined" ? navigator.userAgent : "unknown",
      });
    };

    const onUnhandledRejection = (event: PromiseRejectionEvent) => {
      const reason = event.reason;
      const message = reason instanceof Error ? reason.message : String(reason || "unknown rejection");
      const stack = reason instanceof Error ? reason.stack : undefined;
      const signature = `unhandledrejection:${message}:${stack || ""}`;
      if (!shouldReport(signature)) {
        return;
      }

      sendErrorLog({
        source: "web",
        kind: "unhandledrejection",
        message,
        stack,
        page: typeof window !== "undefined" ? window.location.href : "unknown",
        timestamp: Date.now(),
        userAgent: typeof navigator !== "undefined" ? navigator.userAgent : "unknown",
      });
    };

    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onUnhandledRejection);

    return () => {
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onUnhandledRejection);
    };
  }, []);

  return null;
}
