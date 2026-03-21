import { API_BASE, request } from './request'

/**
 * Checks if a path is a public upload that does not require signing.
 * Based on backend rules:
 * - /uploads/public/...
 * - /uploads/reviews/...
 * - /uploads/merchants/{id}/logo/...
 * - /uploads/merchants/{id}/storefront/...
 * - /uploads/merchants/{id}/environment/...
 */
export function isPublicUploads(urlOrPath: string): boolean {
    if (!urlOrPath) return false
    // Remove leading slash for easier checking ensuring consistency
    const path = urlOrPath.startsWith('/') ? urlOrPath.slice(1) : urlOrPath

    return (
        path.startsWith('uploads/public/') ||
        path.startsWith('uploads/reviews/') ||
        // regex for uploads/merchants/{id}/logo|storefront|environment/
        /^uploads\/merchants\/\d+\/(logo|storefront|environment)\//.test(path)
    )
}

/**
 * Normalizes a path to the stored relative path format (e.g., uploads/...)
 * Returns null if it's an external URL.
 */
function toStoredPath(urlOrPath: string): string | null {
    if (!urlOrPath) return null

    if (/^https?:\/\//.test(urlOrPath)) {
        try {
            const parsedUrl = new URL(urlOrPath)
            const apiBase = new URL(API_BASE)
            const isSameHost = parsedUrl.host === apiBase.host
            if (isSameHost && parsedUrl.pathname.startsWith('/uploads/')) {
                return parsedUrl.pathname.slice(1)
            }
            return null // External URL
        } catch {
            return null
        }
    }

    // Remove leading slash
    return urlOrPath.startsWith('/') ? urlOrPath.slice(1) : urlOrPath
}

export interface SignedUrlResponse {
    url: string
    expires: number
}

// In-memory cache for signed URLs: path -> { url, expires }
const signedUrlCache = new Map<string, SignedUrlResponse>()

/**
 * Resolves an image URL.
 * - If public: returns absolute URL directly.
 * - If private: calls backend to sign and returns signed URL.
 * - If external: returns as is.
 */
export async function resolveImageURL(urlOrPath: string): Promise<string> {
    if (!urlOrPath) return ''

    // 1. External URLs returned as is（同域 uploads 绝对 URL 会在 toStoredPath 中转成可签名路径）
    if (/^https?:\/\//.test(urlOrPath)) {
        // 已经是服务端预签名的 URL（含 sig= 参数），直接使用，避免消费者无凭证二次签名失败
        if (/[?&]sig=/.test(urlOrPath)) {
            return urlOrPath
        }
        const maybeStoredPath = toStoredPath(urlOrPath)
        if (!maybeStoredPath) {
            return urlOrPath
        }
        urlOrPath = maybeStoredPath
    }

    // 2. Public paths: append API_BASE
    if (isPublicUploads(urlOrPath)) {
        // Ensure strictly one slash between base and path
        const baseUrl = API_BASE.endsWith('/') ? API_BASE.slice(0, -1) : API_BASE
        const path = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`
        return `${baseUrl}${path}`
    }

    // 3. Private paths: Sign
    const storedPath = toStoredPath(urlOrPath)
    if (!storedPath) return urlOrPath // Should verify logic here, but if not stored path and not external, what is it? Return as is.

    // Check cache
    const now = Math.floor(Date.now() / 1000)
    const cached = signedUrlCache.get(storedPath)
    // Refresh if expiring in less than 60s
    if (cached && cached.expires > now + 60) {
        return cached.url
    }

    try {
        const res = await request<SignedUrlResponse>({
            url: '/v1/uploads/sign',
            method: 'POST',
            data: { path: '/' + storedPath },
            requestId: `sign_${storedPath}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
            context: 'image-sign'
        })

        // Cache it
        signedUrlCache.set(storedPath, res)
        return res.url

    } catch (e: unknown) {
        // 如果是abort错误，等待短暂时间后检查缓存（另一个并发请求可能成功了）
        if (typeof e === 'object' && e !== null && 'errMsg' in e) {
            const errMsg = (e as { errMsg?: unknown }).errMsg
            if (typeof errMsg === 'string' && errMsg.includes('abort')) {
                await new Promise((resolve) => setTimeout(resolve, 100))
                const cached = signedUrlCache.get(storedPath)
                if (cached && cached.expires > Math.floor(Date.now() / 1000) + 60) {
                    return cached.url
                }
            }
        }

        // 静默处理签名失败，返回备用路径
        const baseUrl = API_BASE.endsWith('/') ? API_BASE.slice(0, -1) : API_BASE
        const path = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`
        return `${baseUrl}${path}`
    }
}

interface PrivateAccessResponse {
    download_url: string
    expire_at: string
}

// In-memory cache for private media access URLs: mediaId -> { url, expireAt }
const privateAccessCache = new Map<number, { url: string, expireAt: number }>()

/**
 * 获取私有媒体资产的短期下载地址（新媒体系统）。
 * 调用 POST /v1/media/private-access。
 */
export async function getPrivateMediaUrl(mediaId: number): Promise<string> {
    if (!mediaId) return ''

    const now = Math.floor(Date.now() / 1000)
    const cached = privateAccessCache.get(mediaId)
    if (cached && cached.expireAt > now + 60) {
        return cached.url
    }

    const res = await request<PrivateAccessResponse>({
        url: '/v1/media/private-access',
        method: 'POST',
        data: { media_id: mediaId }
    })

    const expireAt = Math.floor(new Date(res.expire_at).getTime() / 1000)
    privateAccessCache.set(mediaId, { url: res.download_url, expireAt })
    return res.download_url
}
