"use strict";
/**
 * 预订系统接口
 * 包含创建、查询、取消、确认预订及加菜功能
 * 对应后端 /v1/reservations 路由组
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
exports.markReservationNoShow = exports.completeReservationByMerchant = exports.confirmReservationByMerchant = exports.updateReservation = exports.merchantCreateReservation = exports.getReservationStats = exports.getTodayReservations = exports.getMerchantReservations = exports.startCookingReservation = exports.checkInReservation = exports.addDishesToReservation = exports.cancelReservation = exports.getReservationDetail = exports.getUserReservations = exports.createReservation = exports.ReservationService = void 0;
const request_1 = require("../utils/request");
// ==================== 预订服务 ====================
class ReservationService {
    // ==================== 用户端接口 ====================
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
     * 获取用户预订列表
     * GET /v1/reservations/me
     */
    static getUserReservations(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reservations/me',
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
     * 追加菜品
     * POST /v1/reservations/:id/add-dishes
     */
    static addDishes(id, items) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/add-dishes`,
                method: 'POST',
                data: { items }
            });
        });
    }
    /**
     * 顾客到店签到
     * POST /v1/reservations/:id/checkin
     */
    static checkIn(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/checkin`,
                method: 'POST'
            });
        });
    }
    /**
     * 起菜通知
     * POST /v1/reservations/:id/start-cooking
     */
    static startCooking(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/start-cooking`,
                method: 'POST'
            });
        });
    }
    // ==================== 商户端接口 ====================
    /**
     * 商户获取预订列表
     * GET /v1/reservations/merchant
     */
    static getMerchantReservations(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reservations/merchant',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 商户获取今日预订
     * GET /v1/reservations/merchant/today
     */
    static getTodayReservations() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reservations/merchant/today',
                method: 'GET'
            });
        });
    }
    /**
     * 商户获取预订统计
     * GET /v1/reservations/merchant/stats
     */
    static getReservationStats() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reservations/merchant/stats',
                method: 'GET'
            });
        });
    }
    /**
     * 商户代客创建预订
     * POST /v1/reservations/merchant/create
     */
    static merchantCreateReservation(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reservations/merchant/create',
                method: 'POST',
                data
            });
        });
    }
    /**
     * 商户修改预订
     * PUT /v1/reservations/:id/update
     */
    static updateReservation(id, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/update`,
                method: 'PUT',
                data
            });
        });
    }
    /**
     * 商户确认预订
     * POST /v1/reservations/:id/confirm
     */
    static confirmReservation(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/confirm`,
                method: 'POST'
            });
        });
    }
    /**
     * 商户完成预订
     * POST /v1/reservations/:id/complete
     */
    static completeReservation(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/complete`,
                method: 'POST'
            });
        });
    }
    /**
     * 商户标记未到店
     * POST /v1/reservations/:id/no-show
     */
    static markNoShow(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reservations/${id}/no-show`,
                method: 'POST'
            });
        });
    }
}
exports.ReservationService = ReservationService;
// ==================== 便捷导出函数 ====================
// 用户端
exports.createReservation = ReservationService.createReservation;
exports.getUserReservations = ReservationService.getUserReservations;
exports.getReservationDetail = ReservationService.getReservationDetail;
exports.cancelReservation = ReservationService.cancelReservation;
exports.addDishesToReservation = ReservationService.addDishes;
exports.checkInReservation = ReservationService.checkIn;
exports.startCookingReservation = ReservationService.startCooking;
// 商户端
exports.getMerchantReservations = ReservationService.getMerchantReservations;
exports.getTodayReservations = ReservationService.getTodayReservations;
exports.getReservationStats = ReservationService.getReservationStats;
exports.merchantCreateReservation = ReservationService.merchantCreateReservation;
exports.updateReservation = ReservationService.updateReservation;
exports.confirmReservationByMerchant = ReservationService.confirmReservation;
exports.completeReservationByMerchant = ReservationService.completeReservation;
exports.markReservationNoShow = ReservationService.markNoShow;
exports.default = ReservationService;
