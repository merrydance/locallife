/**
 * 库存管理 API 服务
 */
import { request } from '../utils/request'

// 格式化数字为两位
function pad(n: number): string {
    return n < 10 ? '0' + n : '' + n
}

// 获取今日日期字符串
function getTodayString(): string {
    const today = new Date()
    return `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`
}

// 库存项（带菜品信息）
export interface InventoryItem {
    id: number
    merchant_id: number
    dish_id: number
    dish_name: string
    dish_price: number
    date: string
    total_quantity: number  // -1 表示无限库存
    sold_quantity: number
    available: number
}

// 库存统计
export interface InventoryStats {
    total_dishes: number
    unlimited_dishes: number
    sold_out_dishes: number
    available_dishes: number
}

// 列表响应
interface ListInventoryResponse {
    inventories: InventoryItem[]
}

// 单项响应
interface InventoryResponse {
    id: number
    merchant_id: number
    dish_id: number
    date: string
    total_quantity: number
    sold_quantity: number
    available: number
}

export const InventoryService = {
    /**
     * 获取今日库存列表
     */
    async listTodayInventory(): Promise<InventoryItem[]> {
        const date = getTodayString()
        const response = await request<ListInventoryResponse>({
            url: `/v1/inventory?date=${date}`,
            method: 'GET'
        })
        return response.inventories || []
    },

    /**
     * 获取今日库存统计
     */
    async getTodayStats(): Promise<InventoryStats> {
        const date = getTodayString()
        return await request<InventoryStats>({
            url: `/v1/inventory/stats?date=${date}`,
            method: 'GET'
        })
    },

    /**
     * 创建库存记录
     */
    async createInventory(dishId: number, quantity: number): Promise<InventoryResponse> {
        const date = getTodayString()
        return await request<InventoryResponse>({
            url: '/v1/inventory',
            method: 'POST',
            data: {
                dish_id: dishId,
                date: date,
                total_quantity: quantity
            }
        })
    },

    /**
     * 更新库存数量
     */
    async updateInventory(dishId: number, quantity: number): Promise<InventoryResponse> {
        const date = getTodayString()
        return await request<InventoryResponse>({
            url: `/v1/inventory/${dishId}`,
            method: 'PATCH',
            data: {
                date: date,
                total_quantity: quantity
            }
        })
    },

    /**
     * 创建或更新库存（先尝试创建，如果已存在则更新）
     */
    async setInventory(dishId: number, quantity: number): Promise<InventoryResponse> {
        try {
            // 先尝试创建（大多数菜品没有库存记录）
            return await this.createInventory(dishId, quantity)
        } catch (error: any) {
            // 如果已存在（唯一约束冲突或500错误），则更新
            const errorMsg = error?.message || error?.userMessage || ''
            const isConflict = errorMsg.includes('500') ||
                errorMsg.includes('duplicate') ||
                errorMsg.includes('already exists') ||
                errorMsg.includes('unique constraint')

            if (isConflict) {
                return await this.updateInventory(dishId, quantity)
            }
            throw error
        }
    }
}
