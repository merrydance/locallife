/**
 * 媒体上传 SDK（小程序端）
 *
 * 实现三步上传流程：
 *   1. POST /v1/media/upload-sessions  — 获取预签名上传凭证
 *   2. PUT/POST upload_host            — 直传到 OSS 或本地 dev 端点
 *   3. POST /v1/media/complete         — 通知后端完成，获取 media_id 与 CDN 链接
 */

import { API_BASE } from './request'
import { getToken } from './auth'
import { AppError, ErrorType } from './error-handler'
import { logger } from './logger'

// ==================== 公共类型 ====================

export interface MediaUploadOptions {
  /** 业务归属，如 'merchant' | 'user' | 'rider' | 'operator' | 'group' */
  businessType: string
  /** 媒体分类，如 'logo' | 'dish' | 'combo' | 'table' | 'review' | 'avatar'
   *  | 'business_license' | 'food_permit' | 'id_card_front' | 'id_card_back'
   *  | 'health_cert' */
  mediaCategory: string
  /** MIME 类型，默认 'image/jpeg' */
  contentType?: string
  /** 是否启用压缩（默认 true） */
  compress?: boolean
}

export interface MediaUploadResult {
  mediaId: number
  /** 推荐显示 URL（card 变体，无则取 original） */
  displayUrl: string
  urls: {
    thumb?: string
    card?: string
    detail?: string
    original?: string
  }
}

// ==================== 内部工具 ====================

/** 将十六进制 SHA-256 转换为 base64 */
function hexToBase64(hex: string): string {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.substr(i, 2), 16)
  }
  let binary = ''
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

/** 获取临时文件的 SHA-256（base64）及文件大小 */
function getFileSHA256AndSize(
  filePath: string
): Promise<{ sha256Base64: string, size: number }> {
  return new Promise((resolve) => {
    let size = 0
    try {
      const stat = wx.getFileSystemManager().statSync(filePath) as WechatMiniprogram.Stats
      size = stat.size
    } catch (_e) {
      /* 尺寸未知时以 0 代替 */
    }

    type WxWithFileInfo = WechatMiniprogram.Wx & {
      getFileInfo: (opts: {
        filePath: string,
        digestAlgorithm: string,
        success: (res: { digest: string }) => void,
        fail: () => void,
      }) => void
    }
    (wx as WxWithFileInfo).getFileInfo({
      filePath,
      digestAlgorithm: 'sha256',
      success: (res) => resolve({ sha256Base64: hexToBase64(res.digest), size }),
      fail: () => resolve({ sha256Base64: '', size })
    })
  })
}

/** 可选的图片压缩（失败时静默回退到原路径） */
function compressIfNeeded(filePath: string, compress: boolean): Promise<string> {
  if (!compress) return Promise.resolve(filePath)
  return new Promise((resolve) => {
    wx.compressImage({
      src: filePath,
      quality: 82,
      success: (res) => resolve(res.tempFilePath),
      fail: () => resolve(filePath)
    })
  })
}

// ==================== 三步上传内部实现 ====================

interface CreateSessionRequest {
  business_type: string
  media_category: string
  content_type: string
  content_length: number
  checksum_sha256: string
}

interface CreateSessionResponse {
  upload_id: string
  object_key: string
  visibility: string
  upload_host: string
  form: Record<string, string>
  expire_at: string
}

function createMediaUploadSession(
  req: CreateSessionRequest
): Promise<CreateSessionResponse> {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${API_BASE}/v1/media/upload-sessions`,
      method: 'POST',
      header: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${getToken()}`,
        'X-Response-Envelope': '1'
      },
      data: req,
      success: (res) => {
        const body = res.data as Record<string, unknown>
        if (res.statusCode === 200 || res.statusCode === 201) {
          if (body?.code === 0 && body?.data) {
            resolve(body.data as CreateSessionResponse)
          } else {
            reject(
              new AppError({
                type: ErrorType.BUSINESS,
                message: String(body?.message ?? '创建上传会话失败'),
                userMessage: '上传失败'
              })
            )
          }
        } else {
          reject(
            new AppError({
              type: ErrorType.NETWORK,
              message: `HTTP ${res.statusCode}`,
              userMessage: '上传失败'
            })
          )
        }
      },
      fail: (err) => reject(err)
    })
  })
}

/**
 * 直传文件至 OSS 或本地 dev 端点（不携带 Auth 头）。
 * formFields 由后端 upload-sessions 接口签发，包含 key、签名等字段。
 */
function ossDirectUpload(
  uploadHost: string,
  formFields: Record<string, string>,
  filePath: string
): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.uploadFile({
      url: uploadHost,
      filePath,
      name: 'file',
      formData: formFields as Record<string, unknown>,
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve()
        } else {
          reject(
            new AppError({
              type: ErrorType.NETWORK,
              message: `直传失败: HTTP ${res.statusCode}`,
              userMessage: '上传失败'
            })
          )
        }
      },
      fail: (err) => reject(err)
    })
  })
}

interface CompleteUploadRequest {
  upload_id: string
  object_key: string
  etag?: string
}

interface CompleteUploadResponse {
  media_id: number
  urls: {
    thumb?: string
    card?: string
    detail?: string
    original?: string
  }
  status: string
}

function completeMediaUpload(
  req: CompleteUploadRequest
): Promise<CompleteUploadResponse> {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${API_BASE}/v1/media/complete`,
      method: 'POST',
      header: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${getToken()}`,
        'X-Response-Envelope': '1'
      },
      data: req,
      success: (res) => {
        const body = res.data as Record<string, unknown>
        if (res.statusCode === 200 || res.statusCode === 201) {
          if (body?.code === 0 && body?.data) {
            resolve(body.data as CompleteUploadResponse)
          } else {
            reject(
              new AppError({
                type: ErrorType.BUSINESS,
                message: String(body?.message ?? '完成上传失败'),
                userMessage: '上传失败'
              })
            )
          }
        } else {
          reject(
            new AppError({
              type: ErrorType.NETWORK,
              message: `HTTP ${res.statusCode}`,
              userMessage: '上传失败'
            })
          )
        }
      },
      fail: (err) => reject(err)
    })
  })
}

// ==================== 公开 API ====================

/**
 * 完整三步上传流程：
 *   压缩（可选） → SHA-256 → 创建会话 → 直传 → 完成
 *
 * @returns `{ mediaId, displayUrl, urls }` — displayUrl 是 card 或 original 变体 URL
 */
export async function uploadMedia(
  filePath: string,
  options: MediaUploadOptions
): Promise<MediaUploadResult> {
  const contentType = options.contentType ?? 'image/jpeg'
  const compress = options.compress !== false

  const uploadPath = await compressIfNeeded(filePath, compress)
  const { sha256Base64, size } = await getFileSHA256AndSize(uploadPath)

  logger.debug('uploadMedia: 开始创建上传会话', { options }, 'uploadMedia')

  const session = await createMediaUploadSession({
    business_type: options.businessType,
    media_category: options.mediaCategory,
    content_type: contentType,
    content_length: size,
    checksum_sha256: sha256Base64
  })

  await ossDirectUpload(session.upload_host, session.form, uploadPath)

  const result = await completeMediaUpload({
    upload_id: session.upload_id,
    object_key: session.object_key
  })

  const displayUrl =
    result.urls.card ?? result.urls.detail ?? result.urls.original ?? ''

  logger.debug('uploadMedia: 上传成功', { mediaId: result.media_id }, 'uploadMedia')

  return {
    mediaId: result.media_id,
    displayUrl,
    urls: result.urls
  }
}

/**
 * 发送 application/x-www-form-urlencoded POST 请求（带鉴权头）。
 * 用于将 media_asset_id 传给后端 OCR 端点：
 *   ctx.PostForm("media_asset_id") 可正常读取。
 */
export function postFormData<T = unknown>(
  url: string,
  data: Record<string, string | number>
): Promise<T> {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${API_BASE}${url}`,
      method: 'POST',
      header: {
        'Content-Type': 'application/x-www-form-urlencoded',
        Authorization: `Bearer ${getToken()}`,
        'X-Response-Envelope': '1'
      },
      data,
      success: (res) => {
        const body = res.data as Record<string, unknown>
        if (res.statusCode === 200 || res.statusCode === 201) {
          if (body?.code === 0) {
            resolve(body.data as T)
          } else {
            reject(
              new AppError({
                type: ErrorType.BUSINESS,
                message: String(body?.message ?? '请求失败'),
                userMessage: String(body?.message ?? '操作失败')
              })
            )
          }
        } else {
          reject(
            new AppError({
              type: ErrorType.NETWORK,
              message: `HTTP ${res.statusCode}`,
              userMessage: '操作失败'
            })
          )
        }
      },
      fail: (err) => reject(err)
    })
  })
}

/**
 * 规范化媒体 URL：
 *   - http/https URL 原样返回
 *   - 旧式相对路径 (uploads/...) 添加 API_BASE 前缀
 */
export function getMediaDisplayUrl(url?: string): string {
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://')) return url
  return `${API_BASE}/${url.replace(/^\//, '')}`
}
