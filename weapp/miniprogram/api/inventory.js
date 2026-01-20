"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.InventoryService = void 0;
/**
 * 库存管理 API 服务
 */
const request_1 = require("../utils/request");
// 格式化数字为两位
function pad(n) {
    return n < 10 ? '0' + n : '' + n;
}
// 获取今日日期字符串
function getTodayString() {
    const today = new Date();
    return `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`;
}
exports.InventoryService = {
    /**
     * 获取今日库存列表
     */
    async listTodayInventory() {
        const date = getTodayString();
        const response = await (0, request_1.request)({
            url: `/v1/inventory?date=${date}`,
            method: 'GET'
        });
        return response.inventories || [];
    },
    /**
     * 获取今日库存统计
     */
    async getTodayStats() {
        const date = getTodayString();
        return await (0, request_1.request)({
            url: `/v1/inventory/stats?date=${date}`,
            method: 'GET'
        });
    },
    /**
     * 创建库存记录
     */
    async createInventory(dishId, quantity) {
        const date = getTodayString();
        return await (0, request_1.request)({
            url: '/v1/inventory',
            method: 'POST',
            data: {
                dish_id: dishId,
                date: date,
                total_quantity: quantity
            }
        });
    },
    /**
     * 更新库存数量
     */
    async updateInventory(dishId, quantity) {
        const date = getTodayString();
        return await (0, request_1.request)({
            url: `/v1/inventory/${dishId}`,
            method: 'PATCH',
            data: {
                date: date,
                total_quantity: quantity
            }
        });
    },
    /**
     * 创建或更新库存（先尝试创建，如果已存在则更新）
     */
    async setInventory(dishId, quantity) {
        try {
            // 先尝试创建（大多数菜品没有库存记录）
            return await this.createInventory(dishId, quantity);
        }
        catch (error) {
            // 如果已存在（唯一约束冲突或500错误），则更新
            const errorMsg = (error === null || error === void 0 ? void 0 : error.message) || (error === null || error === void 0 ? void 0 : error.userMessage) || '';
            const isConflict = errorMsg.includes('500') ||
                errorMsg.includes('duplicate') ||
                errorMsg.includes('already exists') ||
                errorMsg.includes('unique constraint');
            if (isConflict) {
                return await this.updateInventory(dishId, quantity);
            }
            throw error;
        }
    }
};
