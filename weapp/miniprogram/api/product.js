"use strict";
/**
 * 商品管理接口
 * 包含商品增删改查、上下架等功能
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
exports.ProductService = void 0;
const request_1 = require("../utils/request");
// ==================== 商品服务 ====================
class ProductService {
    /**
     * 获取商品列表 (商家端)
     * GET /v1/merchant/products
     */
    static getProducts(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/products',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取商品详情
     * GET /v1/merchant/products/:id
     */
    static getProductDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/products/${id}`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建商品
     * POST /v1/merchant/products
     */
    static createProduct(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/products',
                method: 'POST',
                data: data
            });
        });
    }
    /**
     * 更新商品
     * PUT /v1/merchant/products/:id
     */
    static updateProduct(id, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/products/${id}`,
                method: 'PUT',
                data: data
            });
        });
    }
    /**
     * 删除商品
     * DELETE /v1/merchant/products/:id
     */
    static deleteProduct(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/products/${id}`,
                method: 'DELETE'
            });
        });
    }
    /**
     * 更新商品状态
     * POST /v1/merchant/products/:id/status
     */
    static updateProductStatus(id, status) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/products/${id}/status`,
                method: 'POST',
                data: { status }
            });
        });
    }
    /**
     * 获取分类列表
     * GET /v1/merchant/categories
     */
    static getCategories() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/categories',
                method: 'GET'
            });
        });
    }
}
exports.ProductService = ProductService;
exports.default = ProductService;
