"use strict";
/**
 * 收藏系统接口
 * 包含收藏/取消收藏、获取收藏列表
 */
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
exports.FavoriteService = void 0;
const request_1 = require("../utils/request");
// ==================== 收藏服务 ====================
class FavoriteService {
    /**
     * 添加收藏
     * POST /v1/favorites
     */
    static addFavorite(type, targetId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/favorites',
                method: 'POST',
                data: { type, target_id: targetId }
            });
        });
    }
    /**
     * 取消收藏
     * POST /v1/favorites/remove
     */
    static removeFavorite(type, targetId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/favorites/remove',
                method: 'POST',
                data: { type, target_id: targetId }
            });
        });
    }
    /**
     * 获取收藏列表
     * GET /v1/favorites
     */
    static getFavorites(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/favorites',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 检查是否已收藏
     * GET /v1/favorites/check
     */
    static checkFavorite(type, targetId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/favorites/check',
                method: 'GET',
                data: { type, target_id: targetId }
            });
        });
    }
}
exports.FavoriteService = FavoriteService;
exports.default = FavoriteService;
