"use strict";
/**
 * 账单组相关 API
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.createBillingGroup = createBillingGroup;
exports.joinBillingGroup = joinBillingGroup;
exports.listBillingGroups = listBillingGroups;
exports.listBillingGroupOrders = listBillingGroupOrders;
const request_1 = require("../utils/request");
async function createBillingGroup(dining_session_id) {
    return (0, request_1.request)({
        url: '/v1/billing-groups',
        method: 'POST',
        data: { dining_session_id }
    });
}
async function joinBillingGroup(id) {
    return (0, request_1.request)({
        url: `/v1/billing-groups/${id}/join`,
        method: 'POST'
    });
}
async function listBillingGroups(dining_session_id) {
    return (0, request_1.request)({
        url: '/v1/billing-groups',
        method: 'GET',
        data: { dining_session_id }
    });
}
async function listBillingGroupOrders(id) {
    return (0, request_1.request)({
        url: `/v1/billing-groups/${id}/orders`,
        method: 'GET'
    });
}
exports.default = {
    createBillingGroup,
    joinBillingGroup,
    listBillingGroups,
    listBillingGroupOrders
};
