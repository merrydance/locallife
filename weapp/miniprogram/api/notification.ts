import { request } from '../utils/request'
import { normalizePaginatedResult, type PaginatedListResult, type PaginationEnvelope } from './types'

export type NotificationType = 'order' | 'payment' | 'delivery' | 'system' | 'food_safety'

export interface Notification {
    id: number
    user_id: number
    type: NotificationType
    title: string
    content: string
    related_type?: string
    related_id?: number
    extra_data?: Record<string, unknown>
    is_read: boolean
    read_at?: string
    is_pushed: boolean
    pushed_at?: string
    created_at: string
    expires_at?: string
}

export interface ListNotificationsResponse {
    notifications: Notification[]
    total: number
    page_id: number
    page_size: number
}

export interface NotificationListResult extends PaginatedListResult<Notification> {
    notifications: Notification[]
}

export interface NotificationPreferences {
    order_updates?: boolean
    payment_updates?: boolean
    delivery_updates?: boolean
    system_updates?: boolean
    food_safety_updates?: boolean
    quiet_hours_enabled?: boolean
    quiet_hours_start?: string
    quiet_hours_end?: string
}

type NotificationListResponse = PaginationEnvelope & {
    notifications?: Notification[]
}

export class NotificationService {
    async getNotifications(params: { page_id?: number, page_size?: number, type?: NotificationType, is_read?: boolean }): Promise<NotificationListResult> {
        const pageId = params.page_id ?? 1
        const pageSize = params.page_size ?? 20
        const offset = (pageId - 1) * pageSize
        const res = await request<NotificationListResponse>({
            url: '/v1/notifications',
            method: 'GET',
            data: {
                type: params.type,
                is_read: params.is_read,
                limit: pageSize,
                offset
            }
        })

        const notifications = Array.isArray(res?.notifications) ? res.notifications : []
        const normalized = normalizePaginatedResult(notifications, res, { page: pageId, pageSize })

        return {
            ...normalized,
            notifications
        }
    }

    async markAsRead(id: number) {
        return request({
            url: `/v1/notifications/${id}/read`,
            method: 'PUT'
        })
    }

    async markAllAsRead() {
        return request({
            url: `/v1/notifications/read-all`,
            method: 'PUT'
        })
    }

    async getUnreadCount() {
        return request<{ count: number }>({
            url: '/v1/notifications/unread/count',
            method: 'GET'
        })
    }

    async deleteNotification(id: number): Promise<void> {
        await request({
            url: `/v1/notifications/${id}`,
            method: 'DELETE'
        })
    }

    async getPreferences(): Promise<NotificationPreferences> {
        return request({
            url: '/v1/notifications/preferences',
            method: 'GET'
        })
    }

    async updatePreferences(preferences: NotificationPreferences): Promise<NotificationPreferences> {
        return request({
            url: '/v1/notifications/preferences',
            method: 'PUT',
            data: preferences
        })
    }
}

export const notificationService = new NotificationService()
