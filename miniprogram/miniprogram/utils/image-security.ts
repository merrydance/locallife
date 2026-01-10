import { getToken } from './auth'
import { API_BASE } from './request'
import { supabase, SUPABASE_URL } from '../services/supabase'

/**
 * Checks if a path is a public upload that does not require signing.
 */
export function isPublicUploads(urlOrPath: string): boolean {
    if (!urlOrPath) return false
    const path = urlOrPath.startsWith('/') ? urlOrPath.slice(1) : urlOrPath

    // Supabase 公开桶直接允许访问
    if (urlOrPath.includes('/storage/v1/object/public/assets')) return true

    return (
        path.startsWith('uploads/public/') ||
        path.startsWith('uploads/reviews/') ||
        /^uploads\/merchants\/\d+\/(logo|storefront|environment|license|food_permit)\//.test(path)
    )
}

/**
 * Normalizes a path to the stored relative path format (e.g., uploads/...)
 */
function toStoredPath(urlOrPath: string): string | null {
    if (!urlOrPath) return null
    if (/^https?:\/\//.test(urlOrPath)) return null 

    return urlOrPath.startsWith('/') ? urlOrPath.slice(1) : urlOrPath
}

export interface SignedUrlResponse {
    url: string
    expires: number
}

const signedUrlCache = new Map<string, SignedUrlResponse>()

/**
 * Resolves an image URL.
 */
export async function resolveImageURL(urlOrPath: string): Promise<string> {
    if (!urlOrPath) return ''

    // 1. External or already full Supabase URLs
    if (/^https?:\/\//.test(urlOrPath)) return urlOrPath

    // 2. Public paths (Support Legacy and Supabase)
    if (isPublicUploads(urlOrPath)) {
        if (urlOrPath.startsWith('uploads/')) {
            // Mapping legacy to new public bucket endpoint for migration
            return `${SUPABASE_URL}/storage/v1/object/public/assets/${urlOrPath}`
        }
        const baseUrl = API_BASE.endsWith('/') ? API_BASE.slice(0, -1) : API_BASE
        const path = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`
        return `${baseUrl}${path}`
    }

    // 3. Private paths: Sign via Supabase
    const storedPath = toStoredPath(urlOrPath)
    if (!storedPath) return urlOrPath

    const now = Math.floor(Date.now() / 1000)
    const cached = signedUrlCache.get(storedPath)
    if (cached && cached.expires > now + 60) {
        return cached.url
    }

    try {
        const bucket = storedPath.includes('id_card') || storedPath.includes('idcard') ? 'identity' : 'assets'
        const { data, error } = await supabase.storage.from(bucket).createSignedUrl(storedPath, 3600)
        
        if (error || !data) throw error || new Error('Sign failed')

        const res: SignedUrlResponse = { 
            url: data.signedUrl, 
            expires: now + 3600 
        }
        
        signedUrlCache.set(storedPath, res)
        return res.url

    } catch (e: any) {
        // Fallback or retry
        const path = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`
        return `${SUPABASE_URL}/storage/v1/object/public/assets${path}`
    }
}
