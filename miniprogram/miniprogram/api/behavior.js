"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.trackBehavior = trackBehavior;
exports.trackView = trackView;
exports.trackDetail = trackDetail;
exports.trackAddToCart = trackAddToCart;
exports.trackPurchase = trackPurchase;
const request_1 = require("../utils/request");
/**
 * 上报用户行为埋点 - POST /v1/behaviors/track
 */
function trackBehavior(data) {
    return (0, request_1.request)({
        url: '/v1/behaviors/track',
        method: 'POST',
        data
    });
}
/**
 * 便捷方法：追踪浏览行为
 */
function trackView(params) {
    return trackBehavior(Object.assign({ behavior_type: 'view' }, params));
}
/**
 * 便捷方法：追踪详情查看行为
 */
function trackDetail(params) {
    return trackBehavior(Object.assign({ behavior_type: 'detail' }, params));
}
/**
 * 便捷方法：追踪加购行为
 */
function trackAddToCart(params) {
    return trackBehavior(Object.assign({ behavior_type: 'cart' }, params));
}
/**
 * 便捷方法：追踪购买行为
 */
function trackPurchase(params) {
    return trackBehavior(Object.assign({ behavior_type: 'purchase' }, params));
}
