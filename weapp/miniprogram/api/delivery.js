"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getRiderLocation = getRiderLocation;
exports.confirmPickup = confirmPickup;
exports.confirmDelivery = confirmDelivery;
exports.getDeliveryTrack = getDeliveryTrack;
exports.getDeliveryDetail = getDeliveryDetail;
exports.getDeliveryByOrder = getDeliveryByOrder;
const request_1 = require("../utils/request");
/**
 * 获取骑手最新位置
 * GET /v1/delivery/:delivery_id/rider-location
 */
function getRiderLocation(deliveryId) {
    return (0, request_1.request)({
        url: `/v1/delivery/${deliveryId}/rider-location`,
        method: 'GET'
    });
}
/**
 * 骑手确认取餐
 * POST /v1/delivery/:delivery_id/confirm-pickup
 */
function confirmPickup(deliveryId) {
    return (0, request_1.request)({
        url: `/v1/delivery/${deliveryId}/confirm-pickup`,
        method: 'POST'
    });
}
/**
 * 骑手确认送达
 * POST /v1/delivery/:delivery_id/confirm-delivery
 */
function confirmDelivery(deliveryId) {
    return (0, request_1.request)({
        url: `/v1/delivery/${deliveryId}/confirm-delivery`,
        method: 'POST'
    });
}
/**
 * 获取配送轨迹
 * GET /v1/delivery/:delivery_id/track
 */
function getDeliveryTrack(deliveryId, since) {
    return (0, request_1.request)({
        url: `/v1/delivery/${deliveryId}/track`,
        method: 'GET',
        data: since ? { since } : undefined
    });
}
/**
 * 获取配送单详情
 * GET /v1/delivery/:delivery_id
 */
function getDeliveryDetail(deliveryId) {
    return (0, request_1.request)({
        url: `/v1/delivery/${deliveryId}`,
        method: 'GET'
    });
}
/**
 * 根据订单ID获取配送信息
 * GET /v1/delivery/order/:order_id
 */
function getDeliveryByOrder(orderId) {
    return (0, request_1.request)({
        url: `/v1/delivery/order/${orderId}`,
        method: 'GET'
    });
}
