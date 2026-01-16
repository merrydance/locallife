"use strict";
/**
 * 账单组相关 API
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
exports.createBillingGroup = createBillingGroup;
exports.joinBillingGroup = joinBillingGroup;
exports.listBillingGroups = listBillingGroups;
exports.listBillingGroupOrders = listBillingGroupOrders;
const request_1 = require("../utils/request");
function createBillingGroup(dining_session_id) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/billing-groups',
            method: 'POST',
            data: { dining_session_id }
        });
    });
}
function joinBillingGroup(id) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/billing-groups/${id}/join`,
            method: 'POST'
        });
    });
}
function listBillingGroups(dining_session_id) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/billing-groups',
            method: 'GET',
            data: { dining_session_id }
        });
    });
}
function listBillingGroupOrders(id) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/billing-groups/${id}/orders`,
            method: 'GET'
        });
    });
}
exports.default = {
    createBillingGroup,
    joinBillingGroup,
    listBillingGroups,
    listBillingGroupOrders
};
