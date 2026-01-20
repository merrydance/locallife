"use strict";
/**
 * 商品管理接口
 * 包含商品增删改查、上下架等功能
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.ProductService = void 0;
const request_1 = require("../utils/request");
// ==================== 商品服务 ====================
class ProductService {
    /**
     * 获取商品列表 (商家端)
     * GET /v1/merchant/products
     */
    static async getProducts(params) {
        return await (0, request_1.request)({
            url: '/v1/merchant/products',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取商品详情
     * GET /v1/merchant/products/:id
     */
    static async getProductDetail(id) {
        return await (0, request_1.request)({
            url: `/v1/merchant/products/${id}`,
            method: 'GET'
        });
    }
    /**
     * 创建商品
     * POST /v1/merchant/products
     */
    static async createProduct(data) {
        return await (0, request_1.request)({
            url: '/v1/merchant/products',
            method: 'POST',
            data: data
        });
    }
    /**
     * 更新商品
     * PUT /v1/merchant/products/:id
     */
    static async updateProduct(id, data) {
        return await (0, request_1.request)({
            url: `/v1/merchant/products/${id}`,
            method: 'PUT',
            data: data
        });
    }
    /**
     * 删除商品
     * DELETE /v1/merchant/products/:id
     */
    static async deleteProduct(id) {
        return await (0, request_1.request)({
            url: `/v1/merchant/products/${id}`,
            method: 'DELETE'
        });
    }
    /**
     * 更新商品状态
     * POST /v1/merchant/products/:id/status
     */
    static async updateProductStatus(id, status) {
        return await (0, request_1.request)({
            url: `/v1/merchant/products/${id}/status`,
            method: 'POST',
            data: { status }
        });
    }
    /**
     * 获取分类列表
     * GET /v1/merchant/categories
     */
    static async getCategories() {
        return await (0, request_1.request)({
            url: '/v1/merchant/categories',
            method: 'GET'
        });
    }
}
exports.ProductService = ProductService;
exports.default = ProductService;
