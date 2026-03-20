/**
 * media.ts — 媒体上传 SDK
 *
 * 三步流程：
 *   1. createMediaUploadSession  → 后端签发直传凭证
 *   2. ossDirectUpload           → 直接 POST 文件到 OSS（或本地 devupload 代理）
 *   3. completeMediaUpload       → 通知后端完成，获取 media_id + 各尺寸 URL
 */

import { API_BASE, getAuthToken } from "./api";

/* ------------------------------------------------------------------ */
/*  类型定义                                                            */
/* ------------------------------------------------------------------ */

export type MediaVariant = "thumb" | "card" | "detail" | "original";

interface CreateSessionRequest {
  business_type: string;
  media_category: string;
  content_type: string;
  content_length: number;
  checksum_sha256: string;
}

interface UploadSession {
  upload_id: string;
  object_key: string;
  visibility: string;
  upload_host: string;
  form: Record<string, string>;
  expire_at: string;
}

interface CompleteRequest {
  upload_id: string;
  object_key: string;
  etag?: string;
}

interface CompleteResult {
  media_id: number;
  urls: Record<MediaVariant, string>;
  status: string;
}

export interface UploadResult {
  mediaId: number;
  urls: Record<MediaVariant, string>;
}

/* ------------------------------------------------------------------ */
/*  小工具                                                              */
/* ------------------------------------------------------------------ */

async function sha256base64(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  const hashBuffer = await crypto.subtle.digest("SHA-256", buffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  const binary = hashArray.map((b) => String.fromCharCode(b)).join("");
  return btoa(binary);
}

/* ------------------------------------------------------------------ */
/*  步骤 1：向后端申请直传凭证                                          */
/* ------------------------------------------------------------------ */

async function createMediaUploadSession(
  req: CreateSessionRequest
): Promise<UploadSession> {
  const token = getAuthToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Response-Envelope": "1",
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${API_BASE}/media/upload-sessions`, {
    method: "POST",
    headers,
    body: JSON.stringify(req),
    cache: "no-store",
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`upload-sessions failed: ${res.status} ${text}`);
  }

  const json = await res.json();
  // 统一信封格式
  if (json && typeof json.code === "number") {
    if (json.code !== 0) {
      throw new Error(`API_ERROR_${json.code}: ${json.message}`);
    }
    return json.data as UploadSession;
  }
  return json as UploadSession;
}

/* ------------------------------------------------------------------ */
/*  步骤 2：直接向 upload_host POST 文件（兼容 OSS 和本地 devupload）  */
/* ------------------------------------------------------------------ */

async function ossDirectUpload(
  uploadHost: string,
  formFields: Record<string, string>,
  file: File
): Promise<string> {
  const form = new FormData();

  // 按 OSS/devupload 约定：先添加所有表单字段，再添加文件
  for (const [key, value] of Object.entries(formFields)) {
    form.append(key, value);
  }
  form.append("file", file);

  const res = await fetch(uploadHost, {
    method: "POST",
    body: form,
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`direct upload failed: ${res.status} ${text}`);
  }

  // OSS 在 success_action_status=200 时返回空体，ETag 在响应头中
  return res.headers.get("ETag") ?? "";
}

/* ------------------------------------------------------------------ */
/*  步骤 3：通知后端完成上传                                            */
/* ------------------------------------------------------------------ */

async function completeMediaUpload(req: CompleteRequest): Promise<CompleteResult> {
  const token = getAuthToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Response-Envelope": "1",
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${API_BASE}/media/complete`, {
    method: "POST",
    headers,
    body: JSON.stringify(req),
    cache: "no-store",
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`media/complete failed: ${res.status} ${text}`);
  }

  const json = await res.json();
  if (json && typeof json.code === "number") {
    if (json.code !== 0) {
      throw new Error(`API_ERROR_${json.code}: ${json.message}`);
    }
    return json.data as CompleteResult;
  }
  return json as CompleteResult;
}

/* ------------------------------------------------------------------ */
/*  对外入口：uploadMedia                                               */
/* ------------------------------------------------------------------ */

export interface UploadMediaOptions {
  /** 业务类型，例如 "merchant" */
  businessType: string;
  /** 媒体分类，例如 "dish"、"table"、"logo" */
  mediaCategory: string;
}

/**
 * 完整三步上传，返回 { mediaId, urls }。
 */
export async function uploadMedia(
  file: File,
  options: UploadMediaOptions
): Promise<UploadResult> {
  const checksum = await sha256base64(file);

  const session = await createMediaUploadSession({
    business_type: options.businessType,
    media_category: options.mediaCategory,
    content_type: file.type,
    content_length: file.size,
    checksum_sha256: checksum,
  });

  const etag = await ossDirectUpload(session.upload_host, session.form, file);

  const result = await completeMediaUpload({
    upload_id: session.upload_id,
    object_key: session.object_key,
    etag: etag || undefined,
  });

  return { mediaId: result.media_id, urls: result.urls };
}

/* ------------------------------------------------------------------ */
/*  URL 辅助                                                            */
/* ------------------------------------------------------------------ */

const PLACEHOLDER = "/assets/placeholder.png";

/**
 * 从 urls map 中取指定变体的 URL；
 * 兼容旧的 image_url 字符串（以 uploads/ 或 http 开头）。
 */
export function getMediaDisplayUrl(
  url?: string,
  variant: MediaVariant = "card"
): string {
  if (!url) return PLACEHOLDER;
  if (url.startsWith("http")) return url;
  if (url.startsWith("uploads/") || url.startsWith("/uploads/")) {
    return url.startsWith("/") ? url : `/${url}`;
  }
  return PLACEHOLDER;
}

/**
 * 从 uploadMedia 返回的 urls map 中取指定变体的 URL。
 */
export function pickVariantUrl(
  urls: Record<string, string>,
  variant: MediaVariant = "card"
): string {
  return urls[variant] ?? urls["original"] ?? PLACEHOLDER;
}
