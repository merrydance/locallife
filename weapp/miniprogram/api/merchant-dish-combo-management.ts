import {
    BatchUpdateDishStatusRequest,
    CreateDishRequest,
    DishManagementService,
    DishResponse,
    UpdateDishRequest
} from './dish'
import { InventoryService } from './inventory'

export async function getDishes(params: {
    page: number
    page_size: number
    category_id?: number
    keyword?: string
    is_available?: boolean
}): Promise<{ data: DishResponse[]; total: number }> {
    const response = await DishManagementService.listDishes({
        page_id: params.page,
        page_size: params.page_size,
        category_id: params.category_id,
        is_available: params.is_available
    })

    return {
        data: response.dishes || [],
        total: response.total_count || 0
    }
}

export async function createDish(data: CreateDishRequest) {
    return DishManagementService.createDish(data)
}

export async function updateDish(dishId: number, data: UpdateDishRequest) {
    return DishManagementService.updateDish(dishId, data)
}

export async function deleteDish(dishId: number) {
    return DishManagementService.deleteDish(dishId)
}

export async function updateDishStatus(dishId: number, data: { is_online?: boolean; is_available?: boolean }) {
    return DishManagementService.updateDishStatus(dishId, data)
}

export async function batchUpdateDishStatus(data: BatchUpdateDishStatusRequest) {
    return DishManagementService.batchUpdateDishStatus(data)
}

export async function getInventory(dishId: number): Promise<{ available_count: number }> {
    const inventories = await InventoryService.listTodayInventory()
    const item = inventories.find((inv) => inv.dish_id === dishId)
    return { available_count: item ? item.available : 0 }
}

export async function updateInventory(dishId: number, data: { adjustment: number; reason: string }) {
    const inventories = await InventoryService.listTodayInventory()
    const item = inventories.find((inv) => inv.dish_id === dishId)
    const current = item ? item.available : 0
    const nextQuantity = Math.max(0, current + data.adjustment)
    return InventoryService.setInventory(dishId, nextQuantity)
}
