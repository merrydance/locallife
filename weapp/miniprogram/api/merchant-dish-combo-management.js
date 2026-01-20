"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getDishes = getDishes;
exports.createDish = createDish;
exports.updateDish = updateDish;
exports.deleteDish = deleteDish;
exports.updateDishStatus = updateDishStatus;
exports.batchUpdateDishStatus = batchUpdateDishStatus;
exports.getInventory = getInventory;
exports.updateInventory = updateInventory;
const dish_1 = require("./dish");
const inventory_1 = require("./inventory");
async function getDishes(params) {
    const response = await dish_1.DishManagementService.listDishes({
        page_id: params.page,
        page_size: params.page_size,
        category_id: params.category_id,
        is_available: params.is_available
    });
    return {
        data: response.dishes || [],
        total: response.total_count || 0
    };
}
async function createDish(data) {
    return dish_1.DishManagementService.createDish(data);
}
async function updateDish(dishId, data) {
    return dish_1.DishManagementService.updateDish(dishId, data);
}
async function deleteDish(dishId) {
    return dish_1.DishManagementService.deleteDish(dishId);
}
async function updateDishStatus(dishId, data) {
    return dish_1.DishManagementService.updateDishStatus(dishId, data);
}
async function batchUpdateDishStatus(data) {
    return dish_1.DishManagementService.batchUpdateDishStatus(data);
}
async function getInventory(dishId) {
    const inventories = await inventory_1.InventoryService.listTodayInventory();
    const item = inventories.find((inv) => inv.dish_id === dishId);
    return { available_count: item ? item.available : 0 };
}
async function updateInventory(dishId, data) {
    const inventories = await inventory_1.InventoryService.listTodayInventory();
    const item = inventories.find((inv) => inv.dish_id === dishId);
    const current = item ? item.available : 0;
    const nextQuantity = Math.max(0, current + data.adjustment);
    return inventory_1.InventoryService.setInventory(dishId, nextQuantity);
}
