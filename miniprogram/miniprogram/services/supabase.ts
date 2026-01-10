export const SUPABASE_URL = 'https://ls.merrydance.cn'
export const SUPABASE_KEY = 'sb_publishable_ACJWlzQHlZjBrEguHvfOxg_3BJgxAaH'

interface SupabaseRequestOptions {
    url: string // Relative path, e.g., '/rest/v1/dishes' or '/functions/v1/wechat-login'
    method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
    data?: unknown
    headers?: Record<string, string>
}

/**
 * Native Supabase Request Wrapper
 * Replaces supabase-wechat-stable to avoid compatibility issues.
 */
export async function supabaseRequest<T = unknown>(options: SupabaseRequestOptions): Promise<{ data: T | null; error: Error | { message: string } | null }> {
    return new Promise((resolve) => {
        wx.request({
            url: `${SUPABASE_URL}${options.url}`,
            method: (options.method || 'GET') as any,
            data: options.data as any,
            header: {
                'Content-Type': 'application/json',
                'apikey': SUPABASE_KEY,
                'Authorization': options.headers?.['Authorization'] || `Bearer ${SUPABASE_KEY}`,
                ...options.headers
            },
            success: (res) => {
                if (res.statusCode >= 200 && res.statusCode < 300) {
                    resolve({ data: res.data as T, error: null })
                } else {
                    resolve({ data: null, error: (res.data as any) || { message: `HTTP ${res.statusCode}` } })
                }
            },
            fail: (err) => {
                resolve({ data: null, error: err as any })
            }
        })
    })
}

interface PostgrestFilterBuilder<T> {
  eq(column: string, value: string | number | boolean): PostgrestFilterBuilder<T>
  order(column: string, options?: { ascending?: boolean }): PostgrestFilterBuilder<T>
  then(resolve: (result: { data: T[] | null; error: Error | { message: string } | null }) => void): void
}

/**
 * Lightweight Supabase Client Mock
 * Allows using standard syntax: supabase.rpc() and supabase.from().select()
 */
export const supabase = {
    rpc: async <T = any>(name: string, params: Record<string, unknown> = {}): Promise<{ data: T | null; error: Error | { message: string } | null }> => {
        return supabaseRequest<T>({
            url: `/rest/v1/rpc/${name}`,
            method: 'POST',
            data: params
        })
    },
    from: <T = any>(table: string) => ({
        select: (columns: string = '*'): PostgrestFilterBuilder<T> => {
            const builder = {
                _filters: [] as string[],
                _order: '' as string,
                eq(column: string, value: string | number | boolean) {
                    this._filters.push(`${column}=eq.${value}`)
                    return this
                },
                order(column: string, { ascending = true } = {}) {
                    this._order = `${column}.${ascending ? 'asc' : 'desc'}`
                    return this
                },
                then(resolve: (result: { data: T[] | null; error: Error | { message: string } | null }) => void) {
                    const queryStr = builder._filters.length ? `?${builder._filters.join('&')}` : ''
                    const orderStr = builder._order ? `${builder._filters.length ? '&' : '?'}order=${builder._order}` : ''
                    supabaseRequest<T[]>({
                        url: `/rest/v1/${table}${queryStr}${orderStr}`,
                        method: 'GET',
                        headers: { 'Prefer': 'return=representation' }
                    }).then(resolve)
                }
            }
            return builder
        }
    })
}
