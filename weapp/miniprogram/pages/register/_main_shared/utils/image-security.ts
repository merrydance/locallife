import { request } from '../../../../utils/request'

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
