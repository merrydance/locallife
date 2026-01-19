"use strict";
/**
 * 用餐会话相关 API
 * 覆盖预检、开台、会话内下单（堂食/预订到店）
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
exports.precheckDiningSession = precheckDiningSession;
exports.openDiningSession = openDiningSession;
exports.transferDiningSessionTable = transferDiningSessionTable;
exports.createDiningOrder = createDiningOrder;
const request_1 = require("../utils/request");
/** 预检桌台预订占用 */
function precheckDiningSession(tableId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/dining-sessions/precheck',
            method: 'GET',
            data: { table_id: tableId }
        });
    });
}
/** 开启用餐会话（若已存在开放会话，后端会直接返回） */
function openDiningSession(data) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/dining-sessions/open',
            method: 'POST',
            data
        });
    });
}

/** 转台（换桌） */
function transferDiningSessionTable(sessionId, data) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/dining-sessions/${sessionId}/transfer-table`,
            method: 'POST',
            data
        });
    });
}
/** 基于用餐会话创建堂食订单（占位，调用通用订单创建接口） */
function createDiningOrder(payload) {
    return __awaiter(this, void 0, void 0, function* () {
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
    });
}
exports.default = {
    precheckDiningSession,
    openDiningSession,
    transferDiningSessionTable,
    createDiningOrder
};
