import { request } from '../utils/request'

export interface PointsHistoryItem {
    id: string
    type: 'EARN' | 'SPEND'
    amount: number
    description: string
    created_at: string
}

export interface PointsSummary {
    balance: number
    total_earned: number
    total_spent: number
}

export class PointsService {
    /**
     * Get points summary (balance)
     */
    static async getSummary(): Promise<PointsSummary> {
        return request({
            url: '/v1/user/points/summary',
            method: 'GET'
        })
    }

    /**
     * Get points history
     */
    static async getHistory(page: number = 1, pageSize: number = 20): Promise<{ list: PointsHistoryItem[], total: number }> {
        return request({
            url: '/v1/user/points/history',
            method: 'GET',
            data: { page, page_size: pageSize }
        })
    }
}
