"use strict";
/**
 * 用餐会话相关 API
 * 覆盖预检、开台、会话内下单（堂食/预订到店）
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.precheckDiningSession = precheckDiningSession;
exports.openDiningSession = openDiningSession;
exports.transferDiningSessionTable = transferDiningSessionTable;
exports.createDiningOrder = createDiningOrder;
const request_1 = require("../utils/request");
/** 预检桌台预订占用 */
async function precheckDiningSession(tableId) {
    return (0, request_1.request)({
        url: '/v1/dining-sessions/precheck',
        method: 'GET',
        data: { table_id: tableId }
    });
}
/** 开启用餐会话（若已存在开放会话，后端会直接返回） */
async function openDiningSession(data) {
    return (0, request_1.request)({
        url: '/v1/dining-sessions/open',
        method: 'POST',
        data
    });
}
/** 转台（换桌） */
async function transferDiningSessionTable(sessionId, data) {
    return (0, request_1.request)({
        url: `/v1/dining-sessions/${sessionId}/transfer-table`,
        method: 'POST',
        data
    });
}
/** 基于用餐会话创建堂食订单（占位，调用通用订单创建接口） */
async function createDiningOrder(payload) {
    const { merchant_id, table_id, reservation_id, items, notes, order_type = 'dine_in', billing_group_id } = payload;
    return (0, request_1.request)({
        url: '/v1/orders',
        method: 'POST',
        data: {
            merchant_id,
            table_id,
            reservation_id,
            order_type,
            items,
            notes,
            billing_group_id
        }
    });
}
exports.default = {
    precheckDiningSession,
    openDiningSession,
    transferDiningSessionTable,
    createDiningOrder
};
