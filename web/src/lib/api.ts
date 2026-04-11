export const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "/v1";

interface ApiEnvelope<T> {
  code: number;
  message: string;
  data: T;
}

function isEnvelope<T>(value: unknown): value is ApiEnvelope<T> {
  return (
    typeof value === "object" &&
    value !== null &&
    "code" in value &&
    "message" in value
  );
}

function unwrapEnvelope<T>(json: ApiEnvelope<T> | T): T {
  if (isEnvelope<T>(json)) {
    if (json.code !== 0) {
      throw new Error(`API_ERROR_${json.code}: ${json.message || "unknown error"}`);
    }
    return json.data as T;
  }
  return json as T;
}

/* -------------------- Token Management -------------------- */

export function getAuthToken() {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem("access_token");
}

export function getRefreshToken() {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem("refresh_token");
}

function withAuthHeaders(init: RequestInit = {}): Record<string, string> {
  const token = getAuthToken();
  const headers = (init.headers as Record<string, string>) || {};
  if (token) {
    return { ...headers, Authorization: `Bearer ${token}` };
  }
  return headers;
}

/* -------------------- 401 Refresh Logic -------------------- */

let isRefreshing = false;
let failedQueue: Array<{
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
}> = [];

const processQueue = (error: Error | null, token: string | null = null) => {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve(token);
    }
  });
  failedQueue = [];
};

async function tryRefreshToken(): Promise<string> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) {
    throw new Error("AUTH_NO_REFRESH_TOKEN");
  }

  try {
    const response = await fetch(`${API_BASE}/auth/refresh`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Response-Envelope": "1",
      },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    if (!response.ok) {
      if (response.status >= 400 && response.status < 500) {
        throw new Error(`AUTH_REFRESH_FAILED_${response.status}`);
      }
      throw new Error(`SERVER_ERROR_${response.status}`);
    }

    type RefreshPayload = {
      access_token: string;
      refresh_token?: string;
    };
    const json = (await response.json()) as ApiEnvelope<RefreshPayload> | RefreshPayload;
    const data: RefreshPayload = unwrapEnvelope(json);
    
    if (data && data.access_token) {
      window.localStorage.setItem("access_token", data.access_token);
      if (data.refresh_token) {
        window.localStorage.setItem("refresh_token", data.refresh_token);
      }
      return data.access_token;
    }
    
    throw new Error("AUTH_INVALID_RESPONSE");
  } catch (err: unknown) {
    if (err instanceof Error && err.message.includes("AUTH_")) throw err;
    const message = err instanceof Error ? err.message : "UNKNOWN_ERROR";
    throw new Error(`NETWORK_ERROR: ${message}`);
  }
}

/**
 * 统一处理 401 逻辑
 * @param requestFn 重试时调用的函数
 * @param tokenAtTimeOfRequest 发起请求时使用的 token
 */
async function handle401AndRetry<T>(
  requestFn: () => Promise<T>,
  tokenAtTimeOfRequest: string | null
): Promise<T> {
  // 1. 检查当前本地存储的 token 是否已经变化（可能被其他并发请求或标签页刷新了）
  const currentToken = getAuthToken();
  if (currentToken && tokenAtTimeOfRequest && currentToken !== tokenAtTimeOfRequest) {
    // Token 已被更新，直接重试
    return requestFn();
  }

  if (isRefreshing) {
    return new Promise((resolve, reject) => {
      failedQueue.push({ resolve, reject });
    }).then(() => {
      return requestFn();
    });
  }

  isRefreshing = true;

  try {
    const newToken = await tryRefreshToken();
    isRefreshing = false; 
    processQueue(null, newToken);
    return requestFn(); 
  } catch (err: unknown) {
    isRefreshing = false; 
    const errorMsg = err instanceof Error ? err.message : "";
    const isDefinitiveAuthFailure = 
      errorMsg.includes("AUTH_REFRESH_FAILED") || 
      errorMsg.includes("AUTH_NO_REFRESH_TOKEN") ||
      errorMsg.includes("AUTH_INVALID_RESPONSE");

    if (isDefinitiveAuthFailure) {
      processQueue(err instanceof Error ? err : new Error("Unknown error"), null);
      if (typeof window !== "undefined") {
        window.localStorage.removeItem("access_token");
        window.localStorage.removeItem("refresh_token");
        if (!window.location.pathname.includes("/merchant/login")) {
          window.location.href = "/merchant/login";
        }
      }
    } else {
      processQueue(err instanceof Error ? err : new Error("Unknown error"), null);
      throw err instanceof Error ? err : new Error("Unknown error");
    }
    throw err instanceof Error ? err : new Error("Unknown error");
  }
}

function buildQuery(
  params: Record<string, string | number | boolean | undefined>
) {
  const query = Object.entries(params)
    .filter(([, value]) => value !== undefined && value !== "")
    .map(
      ([key, value]) =>
        `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`
    )
    .join("&");

  return query ? `?${query}` : "";
}

async function handleError(response: Response): Promise<never> {
  let errorMsg = `Request failed: ${response.status}`;
  try {
    // 克隆响应以便多次读取（如果需要），虽然这里只读一次
    const errorJson = await response.json();
    const msg =
      (isEnvelope(errorJson) ? errorJson.message : undefined) ||
      errorJson?.message ||
      errorJson?.error ||
      errorJson?.data?.error;
    const code = isEnvelope(errorJson) ? errorJson.code : undefined;
    if (msg) {
      errorMsg += ` - ${code ? `code=${code} ` : ""}${msg}`;
    }
  } catch {}
  throw new Error(errorMsg);
}

/* -------------------- API Methods -------------------- */

export async function apiGet<T>(
  path: string,
  params: Record<string, string | number | boolean | undefined> = {},
  init: RequestInit = {}
): Promise<T> {
  const tokenUsed = getAuthToken();
  const url = `${API_BASE}${path}${buildQuery(params)}`;
  const response = await fetch(url, {
    ...init,
    headers: {
      Accept: "application/json",
      "X-Response-Envelope": "1",
      ...withAuthHeaders(init),
    },
    cache: "no-store",
  });

  if (response.status === 401) {
    return handle401AndRetry(() => apiGet<T>(path, params, init), tokenUsed);
  }

  if (!response.ok) {
    return handleError(response);
  }

  const json = (await response.json()) as ApiEnvelope<T> | T;
  return unwrapEnvelope(json);
}

export async function apiPost<T>(
  path: string,
  body?: unknown,
  init: RequestInit = {}
): Promise<T> {
  const tokenUsed = getAuthToken();
  const url = `${API_BASE}${path}`;
  const response = await fetch(url, {
    ...init,
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      "X-Response-Envelope": "1",
      ...withAuthHeaders(init),
    },
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });

  if (response.status === 401) {
    if (!path.includes("/auth/refresh")) {
      return handle401AndRetry(() => apiPost<T>(path, body, init), tokenUsed);
    }
  }

  if (!response.ok) {
    return handleError(response);
  }

  const json = (await response.json()) as ApiEnvelope<T> | T;
  return unwrapEnvelope(json);
}

export async function apiPut<T>(
  path: string,
  body?: unknown,
  init: RequestInit = {}
): Promise<T> {
  const tokenUsed = getAuthToken();
  const url = `${API_BASE}${path}`;
  const response = await fetch(url, {
    ...init,
    method: "PUT",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      "X-Response-Envelope": "1",
      ...withAuthHeaders(init),
    },
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });

  if (response.status === 401) {
    return handle401AndRetry(() => apiPut<T>(path, body, init), tokenUsed);
  }

  if (!response.ok) {
    return handleError(response);
  }

  const json = (await response.json()) as ApiEnvelope<T> | T;
  return unwrapEnvelope(json);
}

export async function apiPatch<T>(
  path: string,
  body?: unknown,
  init: RequestInit = {}
): Promise<T> {
  const tokenUsed = getAuthToken();
  const url = `${API_BASE}${path}`;
  const response = await fetch(url, {
    ...init,
    method: "PATCH",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      "X-Response-Envelope": "1",
      ...withAuthHeaders(init),
    },
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });

  if (response.status === 401) {
    return handle401AndRetry(() => apiPatch<T>(path, body, init), tokenUsed);
  }

  if (!response.ok) {
    return handleError(response);
  }

  const json = (await response.json()) as ApiEnvelope<T> | T;
  return unwrapEnvelope(json);
}

export async function apiDelete<T>(
  path: string,
  init: RequestInit = {}
): Promise<T> {
  const tokenUsed = getAuthToken();
  const url = `${API_BASE}${path}`;
  const response = await fetch(url, {
    ...init,
    method: "DELETE",
    headers: {
      Accept: "application/json",
      "X-Response-Envelope": "1",
      ...withAuthHeaders(init),
    },
    cache: "no-store",
  });

  if (response.status === 401) {
    return handle401AndRetry(() => apiDelete<T>(path, init), tokenUsed);
  }

  if (!response.ok) {
    return handleError(response);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const contentLength = response.headers.get("content-length");
  if (contentLength === "0") {
    return undefined as T;
  }

  const json = (await response.json()) as ApiEnvelope<T> | T;
  return unwrapEnvelope(json);
}

export function getMediaUrl(path?: string) {
  if (!path) return "";
  if (path.startsWith("http")) return path;
  if (path.startsWith("data:")) return path;

  if (path.startsWith("/dev/uploads/")) {
    return path;
  }

  if (path.startsWith("/uploads/") || path.startsWith("uploads/")) {
    return "";
  }

  return path;
}
const privateAccessCache = new Map<number, { url: string; expireAt: number }>();

export async function getPrivateMediaUrl(mediaId: number): Promise<string> {
  if (!mediaId) return "";

  const now = Math.floor(Date.now() / 1000);
  const cached = privateAccessCache.get(mediaId);
  if (cached && cached.expireAt > now + 60) {
    return cached.url;
  }

  const response = await apiPost<{ download_url: string; expire_at: string }>(
    "/media/private-access",
    { media_id: mediaId }
  );

  const expireAt = Math.floor(new Date(response.expire_at).getTime() / 1000);
  privateAccessCache.set(mediaId, { url: response.download_url, expireAt });
  return response.download_url;
}

export function formatImageUrl(url?: string, _size?: number) {
  void _size;
  if (!url) return "/assets/placeholder.png";
  return url;
}

// Helpers for format (unchanged)
export function formatDate(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function formatAmount(amount?: number) {
  const value = typeof amount === "number" ? amount : 0;
  return (value / 100).toFixed(2);
}

export function formatPercentage(value?: number) {
  const numeric = typeof value === "number" ? value : 0;
  const percent = numeric > 1 ? numeric : numeric * 100;
  return `${percent.toFixed(1)}%`;
}

export function formatGrowthRate(value?: number) {
  const numeric = typeof value === "number" ? value : 0;
  const percent = numeric > 1 ? numeric : numeric * 100;
  const sign = percent > 0 ? "+" : percent < 0 ? "" : "";
  return `${sign}${percent.toFixed(1)}%`;
}

export function getGrowthColor(value?: number) {
  if (!value) return "text-muted-foreground";
  return value > 0 ? "text-emerald-600" : "text-rose-600";
}

export function getRecentRange(days: number) {
  const end = new Date();
  const start = new Date();
  start.setDate(end.getDate() - days);
  return {
    start_date: formatDate(start),
    end_date: formatDate(end),
  };
}
