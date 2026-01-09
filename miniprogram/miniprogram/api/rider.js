"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getRiderDashboard = getRiderDashboard;
exports.getRiderMetrics = getRiderMetrics;
exports.getAvailableOrders = getAvailableOrders;
exports.acceptOrder = acceptOrder;
exports.pickupOrder = pickupOrder;
exports.deliverOrder = deliverOrder;
exports.setRiderOnline = setRiderOnline;
exports.setRiderOffline = setRiderOffline;
const request_1 = require("../utils/request");
/**
 * Get Rider Dashboard Data (Active tasks, deposit info)
 */
function getRiderDashboard() {
    return (0, request_1.request)({
        url: '/rider/dashboard',
        method: 'GET'
    });
}
/**
 * Get Rider Metrics (Today's stats)
 */
function getRiderMetrics() {
    return (0, request_1.request)({
        url: '/rider/metrics/today',
        method: 'GET'
    });
}
/**
 * Get Available Orders (Pool)
 */
function getAvailableOrders(page = 1, pageSize = 20) {
    return (0, request_1.request)({
        url: '/rider/orders/available',
        method: 'GET',
        data: { page, page_size: pageSize }
    });
}
/**
 * Accept an order
 */
function acceptOrder(orderId) {
    return (0, request_1.request)({
        url: `/rider/orders/${orderId}/accept`,
        method: 'POST'
    });
}
/**
 * Pickup an order
 */
function pickupOrder(orderId) {
    return (0, request_1.request)({
        url: `/rider/orders/${orderId}/pickup`,
        method: 'POST'
    });
}
/**
 * Deliver an order
 */
function deliverOrder(orderId) {
    return (0, request_1.request)({
        url: `/rider/orders/${orderId}/deliver`,
        method: 'POST'
    });
}
/**
 * Set Rider Online
 */
function setRiderOnline(mode = 'DELIVERY') {
    return (0, request_1.request)({
        url: '/rider/online',
        method: 'POST',
        data: { mode }
    });
}
/**
 * Set Rider Offline
 */
function setRiderOffline() {
    return (0, request_1.request)({
        url: '/rider/offline',
        method: 'POST'
    });
}
