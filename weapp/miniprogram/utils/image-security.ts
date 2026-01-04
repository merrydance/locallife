import { getToken } from './auth'
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
    if (/^https?:\/\//.test(urlOrPath)) return null // External URL

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

    // 1. External URLs returned as is
    if (/^https?:\/\//.test(urlOrPath)) return urlOrPath

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

    // 调试：临时禁用缓存
    // const now = Math.floor(Date.now() / 1000)
    // const cached = signedUrlCache.get(storedPath)
    // if (cached && cached.expires > now + 60) {
    //     return cached.url
    // }

    try {
        const res = await request<SignedUrlResponse>({
            url: '/v1/uploads/sign',
            method: 'POST',
            data: { path: '/' + storedPath }
        })

        // Cache it
        signedUrlCache.set(storedPath, res)
        return res.url

    } catch (e: any) {
        // 调试：显示签名错误（包含状态码）
        const errorInfo = e?.statusCode
            ? `${e.statusCode}: ${e?.error?.error || e?.message || ''}`
            : (e?.message || e?.errMsg || JSON.stringify(e)?.slice(0, 30))
        wx.showToast({
            title: `签名失败: ${errorInfo?.slice(0, 30)}`,
            icon: 'none',
            duration: 5000
        })


        // 如果是abort错误，等待短暂时间后检查缓存（另一个并发请求可能成功了）
        if (e?.errMsg?.includes('abort')) {
            await new Promise(resolve => setTimeout(resolve, 100))
            const cached = signedUrlCache.get(storedPath)
            if (cached && cached.expires > Math.floor(Date.now() / 1000) + 60) {
                return cached.url
            }
        }

        // 静默处理签名失败，返回备用路径（不打印错误到控制台）
        const baseUrl = API_BASE.endsWith('/') ? API_BASE.slice(0, -1) : API_BASE
        const path = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`
        return `${baseUrl}${path}`
    }
}
