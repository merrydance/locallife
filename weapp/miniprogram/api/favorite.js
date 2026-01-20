"use strict";
/**
 * 收藏系统接口
 * 包含收藏/取消收藏、获取收藏列表
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.FavoriteService = void 0;
const request_1 = require("../utils/request");
// ==================== 收藏服务 ====================
class FavoriteService {
    /**
     * 添加收藏
     * POST /v1/favorites
     */
    static async addFavorite(type, targetId) {
        return await (0, request_1.request)({
            url: '/v1/favorites',
            method: 'POST',
            data: { type, target_id: targetId }
        });
    }
    /**
     * 取消收藏
     * POST /v1/favorites/remove
     */
    static async removeFavorite(type, targetId) {
        return await (0, request_1.request)({
            url: '/v1/favorites/remove',
            method: 'POST',
            data: { type, target_id: targetId }
        });
    }
    /**
     * 获取收藏列表
     * GET /v1/favorites
     */
    static async getFavorites(params) {
        return await (0, request_1.request)({
            url: '/v1/favorites',
            method: 'GET',
            data: params
        });
    }
    /**
     * 检查是否已收藏
     * GET /v1/favorites/check
     */
    static async checkFavorite(type, targetId) {
        return await (0, request_1.request)({
            url: '/v1/favorites/check',
            method: 'GET',
            data: { type, target_id: targetId }
        });
    }
}
exports.FavoriteService = FavoriteService;
exports.default = FavoriteService;
