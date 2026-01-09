"use strict";
/**
 * 地址管理接口
 * 对齐后端 user_address.go
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
exports.AddressService = void 0;
const request_1 = require("../utils/request");
// ==================== 地址服务 ====================
class AddressService {
    /**
     * 获取地址列表
     * GET /v1/addresses
     */
    static getAddresses() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/addresses',
                method: 'GET'
            });
        });
    }
    /**
     * 获取地址详情
     * GET /v1/addresses/:id
     */
    static getAddressDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/addresses/${id}`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建地址
     * POST /v1/addresses
     */
    static createAddress(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/addresses',
                method: 'POST',
                data: data
            });
        });
    }
    /**
     * 更新地址
     * PATCH /v1/addresses/:id (注意：后端用 PATCH 不是 PUT)
     */
    static updateAddress(id, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/addresses/${id}`,
                method: 'PATCH', // 后端期望 PATCH
                data: data
            });
        });
    }
    /**
     * 删除地址
     * DELETE /v1/addresses/:id
     */
    static deleteAddress(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/addresses/${id}`,
                method: 'DELETE'
            });
        });
    }
    /**
     * 设置默认地址
     * PATCH /v1/addresses/:id/default (注意：后端用 PATCH 不是 POST)
     */
    static setDefaultAddress(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/addresses/${id}/default`,
                method: 'PATCH' // 后端期望 PATCH
            });
        });
    }
    /**
     * 获取默认地址
     * 从地址列表中找到 is_default=true 的地址
     */
    static getDefaultAddress() {
        return __awaiter(this, void 0, void 0, function* () {
            const addresses = yield this.getAddresses();
            return addresses.find(a => a.is_default) || addresses[0] || null;
        });
    }
}
exports.AddressService = AddressService;
exports.default = AddressService;
