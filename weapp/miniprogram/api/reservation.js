"use strict";
/**
 * 预订系统接口
 * 包含创建、查询、取消、确认预订及加菜功能
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
exports.ReservationService = void 0;
const request_1 = require("../utils/request");
// ==================== 预订服务 ====================
class ReservationService {
    /**
     * 创建预订
     * POST /v1/reservations
     */
    static createReservation(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reservations',
                method: 'POST',
                data
            });
        });
    }
    /**
     * 获取预订列表
     * GET /v1/reservations
     */
    static getReservations(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/user/reservations',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取预订详情
     * GET /v1/reservations/:id
     */
    static getReservationDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}`,
                method: 'GET'
            });
        });
    }
    /**
     * 取消预订
     * POST /v1/reservations/:id/cancel
     */
    static cancelReservation(id, reason) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/cancel`,
                method: 'POST',
                data: { reason }
            });
        });
    }
    /**
     * 添加预订菜品
     * POST /v1/reservations/:id/items
     */
    static addReservationDishes(id, items) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/items`,
                method: 'POST',
                data: { items }
            });
        });
    }
    // ==================== 商户端接口 ====================
    /**
     * 商户确认预订
     * POST /v1/merchant/reservations/:id/confirm
     */
    static confirmReservation(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/reservations/${id}/confirm`,
                method: 'POST'
            });
        });
    }
    /**
     * 商户拒绝预订
     * POST /v1/merchant/reservations/:id/reject
     */
    static rejectReservation(id, reason) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/reservations/${id}/reject`,
                method: 'POST',
                data: { reason }
            });
        });
    }
    /**
     * 商户标记未到店
     * POST /v1/merchant/reservations/:id/no-show
     */
    static markNoShow(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/reservations/${id}/no-show`,
                method: 'POST'
            });
        });
    }
    /**
     * 商户完成预订
     * POST /v1/merchant/reservations/:id/complete
     */
    static completeReservation(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/reservations/${id}/complete`,
                method: 'POST'
            });
        });
    }
}
exports.ReservationService = ReservationService;
exports.default = ReservationService;
