
import { supabase } from '../services/supabase'
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

        let query = supabase
            .from('dishes')
            .select('*', { count: 'exact' })
            .eq('merchant_id', params.merchant_id)

        if (params.category_id !== undefined) {
            query = query.eq('category_id', params.category_id) // Note: Supabase types might expect string if UUID, but schema said category_id is int? Wait, let's check schema.
        }

        if (params.is_online !== undefined) {
            query = query.eq('is_online', params.is_online)
        }

        // Pagination
        const from = (params.page_id - 1) * params.page_size
        const to = from + params.page_size - 1

        query = query.range(from, to)

        const { data, error, count } = await query

        if (error) {
            console.error('Supabase listDishes error:', error)
            throw error
        }

        return {
            dishes: data || [],
            total_count: count || 0
        }
    }

    /**
     * Get a single dish by ID
     */
    static async getDish(id: string): Promise<Dish | null> {
        const { data, error } = await supabase
            .from('dishes')
            .select('*')
            .eq('id', id)
            .single()

        if (error) {
            console.error('Supabase getDish error:', error)
            throw error
        }

        return data
    }
}
