"use strict";
/**
 * 个人中心功能接口
 * 基于swagger.json完全重构，包含收藏、历史、评价、会员、通知、申诉等功能
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
exports.getFavoriteDishes = getFavoriteDishes;
exports.addDishToFavorites = addDishToFavorites;
exports.removeDishFromFavorites = removeDishFromFavorites;
exports.getFavoriteMerchants = getFavoriteMerchants;
exports.addMerchantToFavorites = addMerchantToFavorites;
exports.removeMerchantFromFavorites = removeMerchantFromFavorites;
exports.getBrowseHistory = getBrowseHistory;
exports.clearBrowseHistory = clearBrowseHistory;
exports.createReview = createReview;
exports.getMyReviews = getMyReviews;
exports.getReviewDetail = getReviewDetail;
exports.getMerchantReviews = getMerchantReviews;
exports.getAllMerchantReviews = getAllMerchantReviews;
exports.replyToReview = replyToReview;
exports.getMyMemberships = getMyMemberships;
exports.getMembershipDetail = getMembershipDetail;
exports.rechargeMembership = rechargeMembership;
exports.getMembershipTransactions = getMembershipTransactions;
exports.getNotifications = getNotifications;
exports.getUnreadNotificationCount = getUnreadNotificationCount;
exports.markNotificationAsRead = markNotificationAsRead;
exports.markAllNotificationsAsRead = markAllNotificationsAsRead;
exports.deleteNotification = deleteNotification;
exports.getNotificationPreferences = getNotificationPreferences;
exports.updateNotificationPreferences = updateNotificationPreferences;
exports.createClaim = createClaim;
exports.getMyClaims = getMyClaims;
exports.getClaimDetail = getClaimDetail;
exports.getMyVouchers = getMyVouchers;
exports.getMyAvailableVouchers = getMyAvailableVouchers;
exports.getAvailableVouchersForMerchant = getAvailableVouchersForMerchant;
exports.claimVoucher = claimVoucher;
exports.isDishFavorited = isDishFavorited;
exports.isMerchantFavorited = isMerchantFavorited;
exports.toggleDishFavorite = toggleDishFavorite;
exports.toggleMerchantFavorite = toggleMerchantFavorite;
exports.getPersonalCenterOverview = getPersonalCenterOverview;
const request_1 = require("../utils/request");
// ==================== 收藏功能接口 ====================
/**
 * 获取收藏的菜品列表
 */
function getFavoriteDishes(params = {}) {
    return (0, request_1.request)({
        url: '/v1/favorites/dishes',
        method: 'GET',
        data: params
    });
}
/**
 * 添加菜品到收藏
 */
function addDishToFavorites(dishId) {
    return (0, request_1.request)({
        url: '/v1/favorites/dishes',
        method: 'POST',
        data: { dish_id: dishId }
    });
}
/**
 * 从收藏中移除菜品
 */
function removeDishFromFavorites(dishId) {
    return (0, request_1.request)({
        url: `/v1/favorites/dishes/${dishId}`,
        method: 'DELETE'
    });
}
/**
 * 获取收藏的商户列表
 */
function getFavoriteMerchants(params = {}) {
    return (0, request_1.request)({
        url: '/v1/favorites/merchants',
        method: 'GET',
        data: params
    });
}
/**
 * 添加商户到收藏
 */
function addMerchantToFavorites(merchantId) {
    return (0, request_1.request)({
        url: '/v1/favorites/merchants',
        method: 'POST',
        data: { merchant_id: merchantId }
    });
}
/**
 * 从收藏中移除商户
 */
function removeMerchantFromFavorites(merchantId) {
    return (0, request_1.request)({
        url: `/v1/favorites/merchants/${merchantId}`,
        method: 'DELETE'
    });
}
// ==================== 浏览历史接口 ====================
/**
 * 获取浏览历史
 */
function getBrowseHistory(params = {}) {
    return (0, request_1.request)({
        url: '/v1/history/browse',
        method: 'GET',
        data: params
    });
}
/**
 * 清空浏览历史
 */
function clearBrowseHistory() {
    return (0, request_1.request)({
        url: '/v1/history/browse',
        method: 'DELETE'
    });
}
// ==================== 评价系统接口 ====================
/**
 * 创建评价
 */
function createReview(data) {
    return (0, request_1.request)({
        url: '/v1/reviews',
        method: 'POST',
        data
    });
}
/**
 * 获取我的评价列表
 */
function getMyReviews(params = {}) {
    return (0, request_1.request)({
        url: '/v1/reviews/me',
        method: 'GET',
        data: params
    });
}
/**
 * 获取评价详情
 */
function getReviewDetail(reviewId) {
    return (0, request_1.request)({
        url: `/v1/reviews/${reviewId}`,
        method: 'GET'
    });
}
/**
 * 获取商户的评价列表
 */
function getMerchantReviews(merchantId, params = {}) {
    return (0, request_1.request)({
        url: `/v1/reviews/merchants/${merchantId}`,
        method: 'GET',
        data: params
    });
}
/**
 * 获取商户的所有评价（包括历史）
 */
function getAllMerchantReviews(merchantId, params = {}) {
    return (0, request_1.request)({
        url: `/v1/reviews/merchants/${merchantId}/all`,
        method: 'GET',
        data: params
    });
}
/**
 * 商户回复评价
 */
function replyToReview(reviewId, data) {
    return (0, request_1.request)({
        url: `/v1/reviews/${reviewId}/reply`,
        method: 'POST',
        data
    });
}
// ==================== 会员系统接口 ====================
/**
 * 获取我的会员卡列表
 */
function getMyMemberships() {
    return (0, request_1.request)({
        url: '/v1/memberships',
        method: 'GET'
    });
}
/**
 * 获取会员卡详情
 */
function getMembershipDetail(membershipId) {
    return (0, request_1.request)({
        url: `/v1/memberships/${membershipId}`,
        method: 'GET'
    });
}
/**
 * 会员卡充值
 */
function rechargeMembership(data) {
    return (0, request_1.request)({
        url: '/v1/memberships/recharge',
        method: 'POST',
        data
    });
}
/**
 * 获取会员卡交易记录
 */
function getMembershipTransactions(membershipId, params = {}) {
    return (0, request_1.request)({
        url: `/v1/memberships/${membershipId}/transactions`,
        method: 'GET',
        data: params
    });
}
// ==================== 通知系统接口 ====================
/**
 * 获取通知列表
 */
function getNotifications(params = {}) {
    return (0, request_1.request)({
        url: '/v1/notifications',
        method: 'GET',
        data: params
    });
}
/**
 * 获取未读通知数量
 */
function getUnreadNotificationCount() {
    return (0, request_1.request)({
        url: '/v1/notifications/unread/count',
        method: 'GET'
    });
}
/**
 * 标记通知为已读
 */
function markNotificationAsRead(notificationId) {
    return (0, request_1.request)({
        url: `/v1/notifications/${notificationId}/read`,
        method: 'PUT'
    });
}
/**
 * 标记所有通知为已读
 */
function markAllNotificationsAsRead() {
    return (0, request_1.request)({
        url: '/v1/notifications/read-all',
        method: 'PUT'
    });
}
/**
 * 删除通知
 */
function deleteNotification(notificationId) {
    return (0, request_1.request)({
        url: `/v1/notifications/${notificationId}`,
        method: 'DELETE'
    });
}
/**
 * 获取通知偏好设置
 */
function getNotificationPreferences() {
    return (0, request_1.request)({
        url: '/v1/notifications/preferences',
        method: 'GET'
    });
}
/**
 * 更新通知偏好设置
 */
function updateNotificationPreferences(data) {
    return (0, request_1.request)({
        url: '/v1/notifications/preferences',
        method: 'PUT',
        data
    });
}
// ==================== 申诉功能接口 ====================
/**
 * 创建申诉
 */
function createClaim(data) {
    return (0, request_1.request)({
        url: '/v1/claims',
        method: 'POST',
        data
    });
}
/**
 * 获取我的申诉列表
 */
function getMyClaims(params = {}) {
    return (0, request_1.request)({
        url: '/v1/claims',
        method: 'GET',
        data: params
    });
}
/**
 * 获取申诉详情
 */
function getClaimDetail(claimId) {
    return (0, request_1.request)({
        url: `/v1/claims/${claimId}`,
        method: 'GET'
    });
}
// ==================== 优惠券接口 ====================
/**
 * 获取我的优惠券列表
 */
function getMyVouchers(params = {}) {
    return (0, request_1.request)({
        url: '/v1/vouchers/me',
        method: 'GET',
        data: params
    });
}
/**
 * 获取我的可用优惠券
 */
function getMyAvailableVouchers() {
    return (0, request_1.request)({
        url: '/v1/vouchers/me/available',
        method: 'GET'
    });
}
/**
 * 获取商户可用优惠券
 */
function getAvailableVouchersForMerchant(merchantId) {
    return (0, request_1.request)({
        url: `/v1/vouchers/available/${merchantId}`,
        method: 'GET'
    });
}
/**
 * 领取优惠券
 */
function claimVoucher(voucherId) {
    return (0, request_1.request)({
        url: `/v1/vouchers/${voucherId}/claim`,
        method: 'POST'
    });
}
// ==================== 便捷方法 ====================
/**
 * 检查菜品是否已收藏
 */
function isDishFavorited(dishId) {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const response = yield getFavoriteDishes({ page: 1, page_size: 1000 });
            return response.dishes.some(dish => dish.dish_id === dishId);
        }
        catch (error) {
            console.error('检查菜品收藏状态失败:', error);
            return false;
        }
    });
}
/**
 * 检查商户是否已收藏
 */
function isMerchantFavorited(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const response = yield getFavoriteMerchants({ page: 1, page_size: 1000 });
            return response.merchants.some(merchant => merchant.merchant_id === merchantId);
        }
        catch (error) {
            console.error('检查商户收藏状态失败:', error);
            return false;
        }
    });
}
/**
 * 切换菜品收藏状态
 */
function toggleDishFavorite(dishId) {
    return __awaiter(this, void 0, void 0, function* () {
        const isFavorited = yield isDishFavorited(dishId);
        if (isFavorited) {
            yield removeDishFromFavorites(dishId);
            return false;
        }
        else {
            yield addDishToFavorites(dishId);
            return true;
        }
    });
}
/**
 * 切换商户收藏状态
 */
function toggleMerchantFavorite(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        const isFavorited = yield isMerchantFavorited(merchantId);
        if (isFavorited) {
            yield removeMerchantFromFavorites(merchantId);
            return false;
        }
        else {
            yield addMerchantToFavorites(merchantId);
            return true;
        }
    });
}
/**
 * 获取个人中心概览数据
 */
function getPersonalCenterOverview() {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const [favoriteDishes, favoriteMerchants, unreadCount, availableVouchers, memberships] = yield Promise.all([
                getFavoriteDishes({ page: 1, page_size: 1 }),
                getFavoriteMerchants({ page: 1, page_size: 1 }),
                getUnreadNotificationCount(),
                getMyAvailableVouchers(),
                getMyMemberships()
            ]);
            return {
                favoriteCount: {
                    dishes: favoriteDishes.total,
                    merchants: favoriteMerchants.total
                },
                unreadNotifications: unreadCount.count,
                availableVouchers: availableVouchers.total,
                membershipCount: memberships.total
            };
        }
        catch (error) {
            console.error('获取个人中心概览失败:', error);
            return {
                favoriteCount: { dishes: 0, merchants: 0 },
                unreadNotifications: 0,
                availableVouchers: 0,
                membershipCount: 0
            };
        }
    });
}
// 兼容性导出
exports.default = {
    // 收藏功能
    getFavoriteDishes,
    addDishToFavorites,
    removeDishFromFavorites,
    getFavoriteMerchants,
    addMerchantToFavorites,
    removeMerchantFromFavorites,
    // 浏览历史
    getBrowseHistory,
    clearBrowseHistory,
    // 评价系统
    createReview,
    getMyReviews,
    getReviewDetail,
    getMerchantReviews,
    getAllMerchantReviews,
    replyToReview,
    // 会员系统
    getMyMemberships,
    getMembershipDetail,
    rechargeMembership,
    getMembershipTransactions,
    // 通知系统
    getNotifications,
    getUnreadNotificationCount,
    markNotificationAsRead,
    markAllNotificationsAsRead,
    deleteNotification,
    getNotificationPreferences,
    updateNotificationPreferences,
    // 申诉功能
    createClaim,
    getMyClaims,
    getClaimDetail,
    // 优惠券
    getMyVouchers,
    getMyAvailableVouchers,
    getAvailableVouchersForMerchant,
    claimVoucher,
    // 便捷方法
    isDishFavorited,
    isMerchantFavorited,
    toggleDishFavorite,
    toggleMerchantFavorite,
    getPersonalCenterOverview
};
