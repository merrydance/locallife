"use strict";
/**
 * 评价系统接口
 * 包含创建评价、查询评价、商家回复等功能
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
exports.ReviewService = void 0;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
// ==================== 评价服务 ====================
class ReviewService {
    /**
     * 创建评价
     * POST /v1/reviews
     */
    static createReview(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reviews',
                method: 'POST',
                data
            });
        });
    }
    /**
     * 获取评价列表
     * GET /v1/reviews
     */
    static getReviews(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/reviews',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取评价详情
     * GET /v1/reviews/:id
     */
    static getReviewDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/reviews/${id}`,
                method: 'GET'
            });
        });
    }
    // ==================== 商家端接口 ====================
    /**
     * 商家回复评价
     * POST /v1/merchant/reviews/:id/reply
     */
    static replyReview(id, content) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/reviews/${id}/reply`,
                method: 'POST',
                data: { content }
            });
        });
    }
    /**
     * 上传评价图片
     * POST /v1/reviews/images/upload
     */
    static uploadReviewImage(filePath) {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve, reject) => {
                const token = (0, auth_1.getToken)();
                wx.uploadFile({
                    url: `${request_1.API_BASE}/v1/reviews/images/upload`,
                    filePath,
                    name: 'image',
                    header: {
                        'Authorization': `Bearer ${token}`
                    },
                    success: (res) => {
                        var _a;
                        if (res.statusCode === 200) {
                            try {
                                const data = JSON.parse(res.data);
                                if (data.code === 0 && data.data && data.data.image_url) {
                                    resolve(data.data.image_url);
                                }
                                else if (data.image_url) {
                                    resolve(data.image_url);
                                }
                                else {
                                    resolve(((_a = data.data) === null || _a === void 0 ? void 0 : _a.image_url) || data.image_url);
                                }
                            }
                            catch (e) {
                                reject(new Error('Parse response failed'));
                            }
                        }
                        else {
                            reject(new Error(`HTTP ${res.statusCode}`));
                        }
                    },
                    fail: reject
                });
            });
        });
    }
}
exports.ReviewService = ReviewService;
exports.default = ReviewService;
