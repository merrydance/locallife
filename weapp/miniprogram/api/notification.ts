import { request } from '../utils/request';

export interface Notification {
    id: number;
    type: 'system' | 'order' | 'promotion';
    title: string;
    content: string;
    is_read: boolean;
    created_at: string;
    action_url?: string;
}

export class NotificationService {
    async getNotifications(params: { page: number; page_size: number; type?: string }) {
        return request<{ list: Notification[]; total: number }>({
            url: '/v1/notifications',
            method: 'GET',
            data: params
        });
    }

    async markAsRead(id: number) {
        return request({
            url: `/v1/notifications/${id}/read`,
            method: 'POST'
        });
    }

    async markAllAsRead() {
        return request({
            url: `/v1/notifications/read-all`,
            method: 'POST'
        });
    }
}

export const notificationService = new NotificationService();
