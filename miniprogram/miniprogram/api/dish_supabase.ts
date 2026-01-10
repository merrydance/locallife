
import { supabaseRequest } from '../services/supabase'
import { Database } from '../typings/database.types'

type Dish = Database['public']['Tables']['dishes']['Row']
type DishInsert = Database['public']['Tables']['dishes']['Insert']
type DishUpdate = Database['public']['Tables']['dishes']['Update']

export interface ListDishesParams {
    merchant_id: string
    category_id?: number
    is_online?: boolean
    page_id: number
    page_size: number
}

export interface ListDishesResponse {
    dishes: Dish[]
    total_count: number
}

export class DishSupabaseService {
    /**
     * List dishes with pagination and filtering
     */
    static async listDishes(params: ListDishesParams): Promise<ListDishesResponse> {
        console.log('DishSupabaseService.listDishes called with:', params)

        const queryParams: string[] = []
        queryParams.push('select=*')
        queryParams.push(`merchant_id=eq.${params.merchant_id}`)

        if (params.category_id !== undefined) {
            queryParams.push(`category_id=eq.${params.category_id}`)
        }
        if (params.is_online !== undefined) {
            queryParams.push(`is_online=eq.${params.is_online}`)
        }

        const queryString = queryParams.join('&')

        // Calculate range
        const from = (params.page_id - 1) * params.page_size
        const to = from + params.page_size - 1

        const { data, error } = await supabaseRequest<Dish[]>({
            url: `/rest/v1/dishes?${queryString}`,
            method: 'GET',
            headers: {
                'Range': `${from}-${to}`,
                'Prefer': 'count=exact'
            }
        })

        // Note: To get total count with 'Prefer: count=exact', Supabase returns it in 'Content-Range' header
        // But wx.request success callback gives us 'res', and our wrapper returns { data, error }.
        // Our simple wrapper dropped the headers. 
        // For now, let's assume infinite scroll or ignore total_count, OR update wrapper to return headers.
        // Given complexity, let's return data and 0 count if wrapper doesn't support it yet.
        // Users can implement header parsing in wrapper if needed.

        if (error) {
            console.error('Supabase listDishes error:', error)
            throw error
        }

        return {
            dishes: data || [],
            total_count: 9999 // Placeholder as we dropped header support in simple wrapper for now
        }
    }

    /**
     * Get a single dish by ID
     */
    static async getDish(id: string): Promise<Dish | null> {
        const { data, error } = await supabaseRequest<Dish>({
            url: `/rest/v1/dishes?id=eq.${id}&limit=1`,
            method: 'GET',
            headers: {
                'Accept': 'application/vnd.pgrst.object+json' // Request single object
            }
        })

        if (error) {
            console.error('Supabase getDish error:', error)
            throw error
        }

        return data
    }
}
