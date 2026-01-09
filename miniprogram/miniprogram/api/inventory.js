"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
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
    listTodayInventory() {
        return __awaiter(this, void 0, void 0, function* () {
            const date = getTodayString();
            const response = yield (0, request_1.request)({
                url: `/v1/inventory?date=${date}`,
                method: 'GET'
            });
            return response.inventories || [];
        });
    },
    /**
     * 获取今日库存统计
     */
    getTodayStats() {
        return __awaiter(this, void 0, void 0, function* () {
            const date = getTodayString();
            return yield (0, request_1.request)({
                url: `/v1/inventory/stats?date=${date}`,
                method: 'GET'
            });
        });
    },
    /**
     * 创建库存记录
     */
    createInventory(dishId, quantity) {
        return __awaiter(this, void 0, void 0, function* () {
            const date = getTodayString();
            return yield (0, request_1.request)({
                url: '/v1/inventory',
                method: 'POST',
                data: {
                    dish_id: dishId,
                    date: date,
                    total_quantity: quantity
                }
            });
        });
    },
    /**
     * 更新库存数量
     */
    updateInventory(dishId, quantity) {
        return __awaiter(this, void 0, void 0, function* () {
            const date = getTodayString();
            return yield (0, request_1.request)({
                url: `/v1/inventory/${dishId}`,
                method: 'PATCH',
                data: {
                    date: date,
                    total_quantity: quantity
                }
            });
        });
    },
    /**
     * 创建或更新库存（先尝试创建，如果已存在则更新）
     */
    setInventory(dishId, quantity) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 先尝试创建（大多数菜品没有库存记录）
                return yield this.createInventory(dishId, quantity);
            }
            catch (error) {
                // 如果已存在（唯一约束冲突或500错误），则更新
                const errorMsg = (error === null || error === void 0 ? void 0 : error.message) || (error === null || error === void 0 ? void 0 : error.userMessage) || '';
                const isConflict = errorMsg.includes('500') ||
                    errorMsg.includes('duplicate') ||
                    errorMsg.includes('already exists') ||
                    errorMsg.includes('unique constraint');
                if (isConflict) {
                    return yield this.updateInventory(dishId, quantity);
                }
                throw error;
            }
        });
    }
};
