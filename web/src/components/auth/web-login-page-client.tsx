"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import QRCode from "qrcode";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useMerchantSession } from "@/components/providers/merchant-session-provider";
import { cn } from "@/lib/utils";

const API_BASE = (process.env.NEXT_PUBLIC_API_BASE?.trim() || "/v1").replace(
  /\/$/,
  ""
);

type WebLoginSessionStatus = {
  code: string;
  status: "pending" | "confirmed" | "consumed" | "expired" | string;
  expires_at: string;
  confirmed_at?: string;
  consumed_at?: string;
  qr_payload?: string;
  poll_token?: string;
};

type WebLoginConsumeResponse = {
  access_token: string;
  access_token_expires_at: string;
  refresh_token: string;
  refresh_token_expires_at: string;
};

async function publicPost<T>(path: string, body?: Record<string, unknown>) {
  const response = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }

  const json = (await response.json()) as { data?: T } | T;
  if (json && typeof json === "object" && "data" in json) {
    return (json as { data?: T }).data as T;
  }
  return json as T;
}

async function publicGet<T>(path: string) {
  const response = await fetch(`${API_BASE}${path}`, {
    method: "GET",
    headers: {
      Accept: "application/json",
    },
    cache: "no-store",
  });

  if (response.status === 204) {
    return null as T;
  }

  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }

  const json = (await response.json()) as { data?: T } | T;
  if (json && typeof json === "object" && "data" in json) {
    return (json as { data?: T }).data as T;
  }
  return json as T;
}

export function WebLoginPageClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [session, setSession] = useState<WebLoginSessionStatus | null>(null);
  const [qrDataUrl, setQrDataUrl] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [isRedirecting, setIsRedirecting] = useState<boolean>(false);
  const [debugLogs, setDebugLogs] = useState<string[]>([]);
  const pollingRef = useRef<number | null>(null);
  const merchantSession = useMerchantSession();
  const consumingRef = useRef(false);
  const lastConsumeAttemptRef = useRef(0);
  const creatingRef = useRef(false);
  const lastCreateAttemptRef = useRef(0);
  const initRef = useRef(false);
  const redirectingRef = useRef(false);
  const pollBackoffRef = useRef(0);
  const consumeRetryTimerRef = useRef<number | null>(null);
  const logPrefix = "[web-login]";
  const debugEnabled = useMemo(
    () => searchParams?.get("debug") === "1",
    [searchParams]
  );

  const appendDebugLog = useCallback(
    (level: "INFO" | "WARN", message: string, data?: unknown) => {
      if (!debugEnabled) return;
      const suffix = data === undefined ? "" : ` ${JSON.stringify(data)}`;
      const entry = `${new Date().toISOString()} ${level} ${message}${suffix}`;
      setDebugLogs((prev) => {
        const next = [...prev, entry];
        return next.length > 200 ? next.slice(next.length - 200) : next;
      });
    },
    [debugEnabled]
  );

  const logInfo = useCallback(
    (message: string, data?: unknown) => {
      if (data === undefined) {
        console.info(logPrefix, message);
      } else {
        console.info(logPrefix, message, data);
      }
      appendDebugLog("INFO", message, data);
    },
    [appendDebugLog]
  );

  const logWarn = useCallback(
    (message: string, data?: unknown) => {
      if (data === undefined) {
        console.warn(logPrefix, message);
      } else {
        console.warn(logPrefix, message, data);
      }
      appendDebugLog("WARN", message, data);
    },
    [appendDebugLog]
  );

  const loginPayload = useMemo(() => {
    if (!session?.code) return "";
    return session.qr_payload || `web-login:${session.code}`;
  }, [session?.code, session?.qr_payload]);

  const createSession = useCallback(async () => {
    if (creatingRef.current) return;
    const now = Date.now();
    if (now - lastCreateAttemptRef.current < 1000) return;
    creatingRef.current = true;
    lastCreateAttemptRef.current = now;
    setLoading(true);
    setError("");
    logInfo("create_session:start");
    if (pollingRef.current) {
      logInfo("polling:clear");
      window.clearTimeout(pollingRef.current);
      pollingRef.current = null;
    }
    try {
      const data = await publicPost<WebLoginSessionStatus>(
        "/auth/web-login/sessions"
      );
      logInfo("create_session:success", {
        code: data.code,
        status: data.status,
        expires_at: data.expires_at,
      });
      if (!data.poll_token) {
        setError("创建登录会话失败，请稍后重试");
        return;
      }
      setSession(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logWarn("create_session:error", message);
      if (message.includes("429")) {
        setError("请求过于频繁，请稍后重试");
      } else {
        setError("创建登录会话失败，请稍后重试");
      }
    } finally {
      creatingRef.current = false;
      setLoading(false);
    }
  }, [logInfo, logWarn]);

  const consumeSession = useCallback(async (pollToken: string) => {
    try {
      logInfo("consume:start");
      const result = await publicPost<WebLoginConsumeResponse>(
        "/auth/web-login/consume",
        { poll_token: pollToken }
      );
      logInfo("consume:success", {
        access_token_expires_at: result.access_token_expires_at,
        refresh_token_expires_at: result.refresh_token_expires_at,
      });
      if (consumeRetryTimerRef.current) {
        window.clearTimeout(consumeRetryTimerRef.current);
        consumeRetryTimerRef.current = null;
      }
      window.localStorage.setItem("access_token", result.access_token);
      window.localStorage.setItem("refresh_token", result.refresh_token);
      setIsRedirecting(true);
      try {
        logInfo("merchant_session:refresh");
        await merchantSession?.refresh();
        logInfo("redirect:replace", "/merchant/dashboard");
        router.replace("/merchant/dashboard");
        logInfo("redirect:hard", "/merchant/dashboard");
        window.location.href = "/merchant/dashboard";
      } catch {
        logWarn("merchant_session:refresh_failed");
        window.localStorage.removeItem("access_token");
        window.localStorage.removeItem("refresh_token");
        setIsRedirecting(false);
        setError("当前账号不是商户主账号，无法登录工作台");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logWarn("consume:error", message);
      if (message.includes("429")) {
        setError("请求过于频繁，正在稍后重试...");
        if (!consumeRetryTimerRef.current) {
          consumeRetryTimerRef.current = window.setTimeout(() => {
            consumeRetryTimerRef.current = null;
            consumingRef.current = false;
            void consumeSession(pollToken);
          }, 5000);
        }
        return;
      }
      if (!message.includes("429")) {
        setError("登录失败，请重试");
      }
      consumingRef.current = false;
    }
  }, [router, merchantSession, logInfo, logWarn]);

  const handleStatus = useCallback(async (status: WebLoginSessionStatus) => {
    logInfo("status:update", {
      code: status.code,
      status: status.status,
      confirmed_at: status.confirmed_at,
      consumed_at: status.consumed_at,
    });
    setSession((prev) => ({
      ...status,
      poll_token: prev?.poll_token ?? status.poll_token,
    }));
    if (status.status === "confirmed") {
      const now = Date.now();
      if (consumingRef.current) return;
      if (now - lastConsumeAttemptRef.current < 3000) return;
      const pollToken = session?.poll_token;
      if (!pollToken) {
        setError("登录会话异常，请刷新二维码");
        return;
      }
      consumingRef.current = true;
      lastConsumeAttemptRef.current = now;
      await consumeSession(pollToken);
    }
    if (status.status === "consumed") {
      setError("登录已在其他设备完成，请刷新二维码");
      if (pollingRef.current) {
        window.clearTimeout(pollingRef.current);
        pollingRef.current = null;
      }
    }
    if (status.status === "expired") {
      setError("二维码已过期，请刷新");
      if (pollingRef.current) {
        window.clearTimeout(pollingRef.current);
        pollingRef.current = null;
      }
    }
  }, [consumeSession, session?.poll_token, logInfo]);

  const longPollStatus = useCallback(
    async (code: string, pollToken: string, lastStatus?: string) => {
      const params = new URLSearchParams();
      params.set("poll_token", pollToken);
      params.set("wait", "25");
      if (lastStatus) {
        params.set("last_status", lastStatus);
      }
      const status = await publicGet<WebLoginSessionStatus | null>(
        `/auth/web-login/sessions/${encodeURIComponent(code)}?${params.toString()}`
      );
      return status;
    },
    []
  );

  useEffect(() => {
    if (!initRef.current) {
      initRef.current = true;
      logInfo("init:create_session");
      void createSession();
    }
    return () => {
      if (pollingRef.current) {
        logInfo("cleanup:polling");
        window.clearTimeout(pollingRef.current);
      }
    };
  }, [createSession, logInfo]);

  useEffect(() => {
    if (redirectingRef.current) return;
    if (typeof window === "undefined") return;
    const token = window.localStorage.getItem("access_token");
    if (!token) return;
    redirectingRef.current = true;
    setIsRedirecting(true);
    logInfo("auto_redirect:token_found");
    Promise.resolve(merchantSession?.refresh())
      .then(() => {
        logInfo("auto_redirect:success", "/merchant/dashboard");
        window.location.href = "/merchant/dashboard";
      })
      .catch(() => {
        logWarn("auto_redirect:failed");
        redirectingRef.current = false;
        setIsRedirecting(false);
      });
  }, [merchantSession, logInfo, logWarn]);

  useEffect(() => {
    if (!loginPayload) return;
    logInfo("qr:render", { payload: loginPayload });
    QRCode.toDataURL(loginPayload, { width: 240, margin: 1 }).then(setQrDataUrl);
  }, [loginPayload, logInfo]);

  useEffect(() => {
    if (!session?.code || !session.poll_token) return;
    let active = true;
    let lastStatus = session.status || "pending";
    pollBackoffRef.current = 0;

    const sleep = (ms: number) =>
      new Promise<void>((resolve) => {
        pollingRef.current = window.setTimeout(() => {
          pollingRef.current = null;
          resolve();
        }, ms);
      });

    const loop = async () => {
      while (active) {
        if (consumingRef.current) {
          await sleep(1000);
          continue;
        }
        try {
          logInfo("poll:long_start", { code: session.code, last_status: lastStatus });
          const status = await longPollStatus(
            session.code!,
            session.poll_token!,
            lastStatus
          );
          if (!active) return;
          if (status) {
            logInfo("poll:success", {
              code: status.code,
              status: status.status,
            });
            pollBackoffRef.current = 0;
            await handleStatus(status);
            lastStatus = status.status;
            if (status.status === "consumed" || status.status === "expired") {
              return;
            }
          } else {
            logInfo("poll:timeout");
          }
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          logWarn("poll:error", message);
          if (message.includes("429")) {
            pollBackoffRef.current = Math.min(
              pollBackoffRef.current ? pollBackoffRef.current * 2 : 4000,
              20000
            );
            const delay = pollBackoffRef.current || 4000;
            logWarn("poll:backoff", { delay });
            await sleep(delay);
            continue;
          }
          setError("轮询登录状态失败");
          await sleep(3000);
        }
      }
    };

    void loop();

    return () => {
      active = false;
      if (pollingRef.current) {
        window.clearTimeout(pollingRef.current);
        pollingRef.current = null;
      }
      if (consumeRetryTimerRef.current) {
        window.clearTimeout(consumeRetryTimerRef.current);
        consumeRetryTimerRef.current = null;
      }
    };
  }, [session?.code, session?.poll_token, session?.status, handleStatus, longPollStatus, logInfo, logWarn]);

  return (
    <div className="min-h-screen bg-muted/30">
      <div className="mx-auto flex min-h-screen max-w-5xl items-center justify-center px-6">
        <div className="grid w-full gap-8 rounded-2xl bg-white p-10 shadow-xl lg:grid-cols-[1.1fr_0.9fr]">
          <div className="space-y-6">
            <div>
              <div className="text-sm text-primary">商户工作台登录</div>
              <h1 className="mt-2 text-2xl font-semibold text-slate-900">
                使用小程序扫码登录
              </h1>
              <p className="mt-2 text-sm text-slate-500">
                打开小程序「用户中心」扫码登录，确认后即可进入 Web 工作台。
              </p>
            </div>

            <div className="rounded-xl border bg-muted/20 p-4 text-sm text-slate-600">
              <div>登录步骤：</div>
              <ol className="mt-2 list-decimal space-y-1 pl-5">
                <li>打开小程序进入「用户中心」</li>
                <li>点击扫码并对准右侧二维码</li>
                <li>在小程序中确认登录</li>
              </ol>
            </div>

            {isRedirecting ? (
              <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
                登录成功，正在跳转…
              </div>
            ) : error ? (
              <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-600">
                {error}
              </div>
            ) : null}

            {debugEnabled ? (
              <div className="rounded-lg border bg-slate-50 p-3 text-xs text-slate-600">
                <div className="mb-2 flex items-center justify-between">
                  <span className="font-medium text-slate-700">调试日志</span>
                  <button
                    type="button"
                    className="text-xs text-slate-500 hover:text-slate-700"
                    onClick={() => setDebugLogs([])}
                  >
                    清空
                  </button>
                </div>
                <div className="max-h-60 overflow-auto whitespace-pre-wrap break-all font-mono">
                  {debugLogs.length ? debugLogs.join("\n") : "暂无日志"}
                </div>
              </div>
            ) : null}

            <div className="flex items-center gap-3">
              <Button onClick={createSession} disabled={loading}>
                {loading ? "生成中..." : "刷新二维码"}
              </Button>
              <span className="text-xs text-slate-400">
                二维码有效期约 5 分钟
              </span>
            </div>
          </div>

          <div className="flex flex-col items-center justify-center gap-4 rounded-xl border bg-slate-50 p-6">
            <div className="relative group overflow-hidden rounded-xl bg-white shadow">
              {qrDataUrl ? (
                <>
                  <img
                    src={qrDataUrl}
                    alt="Web 登录二维码"
                    className={cn(
                      "h-60 w-60 p-3 transition-all duration-300",
                      session?.status === "expired" && "blur-[6px] opacity-40 scale-105"
                    )}
                  />
                  {session?.status === "expired" && (
                    <div
                      className="absolute inset-0 flex flex-col items-center justify-center cursor-pointer bg-black/5 hover:bg-black/10 transition-colors"
                      onClick={() => void createSession()}
                    >
                      <div className="rounded-full bg-white/90 p-3 shadow-lg text-primary hover:scale-110 transition-transform">
                        <RefreshCw className={cn("h-8 w-8", loading && "animate-spin")} />
                      </div>
                      <span className="mt-4 text-sm font-medium text-slate-800 bg-white/80 px-3 py-1 rounded-full shadow-sm">
                        二维码已过期，点击刷新
                      </span>
                    </div>
                  )}
                </>
              ) : (
                <div className="flex h-60 w-60 items-center justify-center text-sm text-slate-400">
                  {loading ? (
                    <div className="flex flex-col items-center gap-2">
                      <RefreshCw className="h-6 w-6 animate-spin text-primary/50" />
                      <span>正在生成...</span>
                    </div>
                  ) : "二维码加载失败"}
                </div>
              )}
            </div>
            <div className="text-xs text-slate-400">登录码：{session?.code ?? "-"}</div>
            <div className="text-xs text-slate-400">
              状态：{session?.status ?? "pending"}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
