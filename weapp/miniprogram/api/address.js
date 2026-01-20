"use strict";
/**
 * 地址管理接口
 * 对齐后端 user_address.go
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.AddressService = void 0;
const request_1 = require("../utils/request");
// ==================== 地址服务 ====================
class AddressService {
    /**
     * 获取地址列表
     * GET /v1/addresses
     */
    static async getAddresses() {
        return await (0, request_1.request)({
            url: '/v1/addresses',
            method: 'GET'
        });
    }
    /**
     * 获取地址详情
     * GET /v1/addresses/:id
     */
    static async getAddressDetail(id) {
        return await (0, request_1.request)({
            url: `/v1/addresses/${id}`,
            method: 'GET'
        });
    }
    /**
     * 创建地址
     * POST /v1/addresses
     */
    static async createAddress(data) {
        return await (0, request_1.request)({
            url: '/v1/addresses',
            method: 'POST',
            data: data
        });
    }
    /**
     * 更新地址
     * PATCH /v1/addresses/:id (注意：后端用 PATCH 不是 PUT)
     */
    static async updateAddress(id, data) {
        return await (0, request_1.request)({
            url: `/v1/addresses/${id}`,
            method: 'PATCH', // 后端期望 PATCH
            data: data
        });
    }
    /**
     * 删除地址
     * DELETE /v1/addresses/:id
     */
    static async deleteAddress(id) {
        return await (0, request_1.request)({
            url: `/v1/addresses/${id}`,
            method: 'DELETE'
        });
    }
    /**
     * 设置默认地址
     * PATCH /v1/addresses/:id/default (注意：后端用 PATCH 不是 POST)
     */
    static async setDefaultAddress(id) {
        return await (0, request_1.request)({
            url: `/v1/addresses/${id}/default`,
            method: 'PATCH' // 后端期望 PATCH
        });
    }
    /**
     * 获取默认地址
     * 从地址列表中找到 is_default=true 的地址
     */
    static async getDefaultAddress() {
        const addresses = await this.getAddresses();
        return addresses.find(a => a.is_default) || addresses[0] || null;
    }
}
exports.AddressService = AddressService;
exports.default = AddressService;
