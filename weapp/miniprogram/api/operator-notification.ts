import { request } from '../utils/request'
import { normalizePaginatedResult, type PaginatedListResult, type PaginationEnvelope } from './types'

export type OperatorNotificationCategory = 'dispatch_timeout' | 'system'

export interface OperatorNotification {
  id: number
  type: string
  category: OperatorNotificationCategory
  level?: string
  title: string
  content: string
  summary?: string
  related_type?: string
  related_id?: number
  region_id?: number
  region_name?: string
  wait_minutes?: number
  is_read: boolean
  read_at?: string
  created_at: string
  expires_at?: string
}

export interface OperatorNotificationSummaryResponse {
  unread_count: number
  latest_notification?: OperatorNotification
}

export interface OperatorNotificationListResult extends PaginatedListResult<OperatorNotification> {
  notifications: OperatorNotification[]
}

type OperatorNotificationListResponse = PaginationEnvelope & {
  notifications?: OperatorNotification[]
}

export class OperatorNotificationService {
  async getNotifications(params: {
    page_id?: number
    page_size?: number
    category?: OperatorNotificationCategory
    is_read?: boolean
  }): Promise<OperatorNotificationListResult> {
    const pageId = params.page_id ?? 1
    const pageSize = params.page_size ?? 20
    const offset = (pageId - 1) * pageSize
    const res = await request<OperatorNotificationListResponse>({
      url: '/v1/operators/me/notifications',
      method: 'GET',
      data: {
        category: params.category,
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

  async getSummary() {
    return request<OperatorNotificationSummaryResponse>({
      url: '/v1/operators/me/notifications/summary',
      method: 'GET'
    })
  }

  async getDetail(id: number) {
    return request<OperatorNotification>({
      url: `/v1/operators/me/notifications/${id}`,
      method: 'GET'
    })
  }

  async markAsRead(id: number) {
    return request<OperatorNotification>({
      url: `/v1/operators/me/notifications/${id}/read`,
      method: 'PUT'
    })
  }

  async markAllAsRead() {
    return request<{ success: boolean }>({
      url: '/v1/operators/me/notifications/read-all',
      method: 'PUT'
    })
  }
}

export const operatorNotificationService = new OperatorNotificationService()