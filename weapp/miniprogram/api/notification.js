"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.notificationService = exports.NotificationService = void 0;
const request_1 = require("../utils/request");
class NotificationService {
    async getNotifications(params) {
        var _a, _b;
        const pageId = (_a = params.page_id) !== null && _a !== void 0 ? _a : 1;
        const pageSize = (_b = params.page_size) !== null && _b !== void 0 ? _b : 20;
        const offset = (pageId - 1) * pageSize;
        return (0, request_1.request)({
            url: '/v1/notifications',
            method: 'GET',
            data: {
                type: params.type,
                is_read: params.is_read,
                limit: pageSize,
                offset
            }
        });
    }
    async markAsRead(id) {
        return (0, request_1.request)({
            url: `/v1/notifications/${id}/read`,
            method: 'PUT'
        });
    }
    async markAllAsRead() {
        return (0, request_1.request)({
            url: `/v1/notifications/read-all`,
            method: 'PUT'
        });
    }
    async getUnreadCount() {
        return (0, request_1.request)({
            url: '/v1/notifications/unread/count',
            method: 'GET'
        });
    }
}
exports.NotificationService = NotificationService;
exports.notificationService = new NotificationService();
