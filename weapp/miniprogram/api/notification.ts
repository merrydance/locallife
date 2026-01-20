import { request } from '../utils/request';

export type NotificationType = 'order' | 'payment' | 'delivery' | 'system' | 'food_safety'

export interface Notification {
    id: number;
    user_id: number;
    type: NotificationType;
    title: string;
    content: string;
    related_type?: string;
    related_id?: number;
    extra_data?: Record<string, unknown>;
    is_read: boolean;
    read_at?: string;
    is_pushed: boolean;
    pushed_at?: string;
    created_at: string;
    expires_at?: string;
}

export interface ListNotificationsResponse {
    notifications: Notification[];
    total_count: number;
    total: number;
    page_id: number;
    page_size: number;
}

export class NotificationService {
    async getNotifications(params: { page_id?: number; page_size?: number; type?: NotificationType; is_read?: boolean }) {
        const pageId = params.page_id ?? 1
        const pageSize = params.page_size ?? 20
        const offset = (pageId - 1) * pageSize
        return request<ListNotificationsResponse>({
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

    async markAsRead(id: number) {
        return request({
            url: `/v1/notifications/${id}/read`,
            method: 'PUT'
        });
    }

    async markAllAsRead() {
        return request({
            url: `/v1/notifications/read-all`,
            method: 'PUT'
        });
    }

    async getUnreadCount() {
        return request<{ count: number }>({
            url: '/v1/notifications/unread/count',
            method: 'GET'
        })
    }
}

export const notificationService = new NotificationService();
