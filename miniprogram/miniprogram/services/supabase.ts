export const SUPABASE_URL = 'https://ls.merrydance.cn'
export const SUPABASE_KEY = 'sb_publishable_ACJWlzQHlZjBrEguHvfOxg_3BJgxAaH'

interface SupabaseRequestOptions {
    url: string // Relative path, e.g., '/rest/v1/dishes' or '/functions/v1/wechat-login'
    method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
    data?: unknown
    headers?: Record<string, string>
}

import { getToken } from '../utils/auth'

/**
 * Native Supabase Request Wrapper
 */
export async function supabaseRequest<T = unknown>(options: SupabaseRequestOptions): Promise<{ data: T | null; error: Error | { message: string } | null }> {
    const token = getToken()
    const authHeader = token ? `Bearer ${token}` : `Bearer ${SUPABASE_KEY}`

    return new Promise((resolve) => {
        wx.request({
            url: `${SUPABASE_URL}${options.url}`,
            method: (options.method || 'GET') as any,
            data: options.data as any,
            header: {
                'Content-Type': 'application/json',
                'apikey': SUPABASE_KEY,
                'Authorization': options.headers?.['Authorization'] || authHeader,
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

interface PostgrestFilterBuilder<T> extends PromiseLike<{ data: T[] | null; error: Error | { message: string } | null }> {
  eq(column: string, value: string | number | boolean): PostgrestFilterBuilder<T>
  order(column: string, options?: { ascending?: boolean }): PostgrestFilterBuilder<T>
  single(): Promise<{ data: T | null; error: Error | { message: string } | null }>
}

/**
 * Lightweight Supabase Client Mock
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
            const builder: any = {
                _filters: [] as string[],
                _order: '' as string,
                _single: false,
                eq(column: string, value: string | number | boolean) {
                    this._filters.push(`${column}=eq.${value}`)
                    return this
                },
                order(column: string, { ascending = true } = {}) {
                    this._order = `${column}.${ascending ? 'asc' : 'desc'}`
                    return this
                },
                async single() {
                    this._single = true
                    const res = await (this as any)
                    return { ...res, data: res.data ? res.data[0] : null }
                },
                then(onfulfilled?: any, onrejected?: any) {
                    const queryStr = builder._filters.length ? `?${builder._filters.join('&')}` : ''
                    const orderStr = builder._order ? `${builder._filters.length ? '&' : '?'}order=${builder._order}` : ''
                    return supabaseRequest<T[]>({
                        url: `/rest/v1/${table}${queryStr}${orderStr}`,
                        method: 'GET',
                        headers: { 
                            'Prefer': builder._single ? 'return=representation,count=exact' : 'return=representation' 
                        }
                    }).then(onfulfilled, onrejected)
                }
            }
            return builder
        },
        insert: async (data: Partial<T> | Partial<T>[]): Promise<{ data: T[] | null; error: any }> => {
            return supabaseRequest<T[]>({
                url: `/rest/v1/${table}`,
                method: 'POST',
                headers: { 'Prefer': 'return=representation' },
                data
            })
        },
        update: (data: Partial<T>) => ({
            eq: async (column: string, value: string | number | boolean): Promise<{ data: T[] | null; error: any }> => {
                return supabaseRequest<T[]>({
                    url: `/rest/v1/${table}?${column}=eq.${value}`,
                    method: 'PATCH',
                    headers: { 'Prefer': 'return=representation' },
                    data
                })
            }
        }),
        upsert: async (data: Partial<T> | Partial<T>[]): Promise<{ data: T[] | null; error: any }> => {
            return supabaseRequest<T[]>({
                url: `/rest/v1/${table}`,
                method: 'POST',
                headers: { 'Prefer': 'return=representation,resolution=merge-duplicates' },
                data
            })
        }
    }),
    storage: {
        from: (bucket: string) => ({
            getPublicUrl: (path: string) => ({
                data: { publicUrl: `${SUPABASE_URL}/storage/v1/object/public/${bucket}/${path}` }
            }),
            createSignedUrl: async (path: string, expiresIn: number = 3600): Promise<{ data: { signedUrl: string } | null; error: Error | null }> => {
                const { data, error } = await supabaseRequest<{ signedURL: string }>({
                    url: `/storage/v1/object/sign/${bucket}/${path}`,
                    method: 'POST',
                    data: { expiresIn }
                })
                return { 
                    data: data ? { signedUrl: `${SUPABASE_URL}${data.signedURL}` } : null,
                    error: error as any 
                }
            }
        })
    }
}
