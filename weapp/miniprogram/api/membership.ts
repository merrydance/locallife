import { request } from '../utils/request'

export interface Membership {
    id: number
    merchant_id: number
    merchant_name?: string
    logo_url?: string
    user_id: number
    balance: number
    total_recharged: number
    total_consumed: number
    created_at: string
}

export interface ListMembershipsResponse {
    memberships: Membership[]
    total: number // Backend also sends this
    page_id: number
    page_size: number
}

export class MembershipService {
    /**
     * 获取用户会员卡列表
     * GET /v1/memberships
     */
    static async listMyMemberships(pageId: number = 1, pageSize: number = 20): Promise<ListMembershipsResponse> {
        return await request({
            url: '/v1/memberships',
            method: 'GET',
            data: { page_id: pageId, page_size: pageSize }
        })
    }

    /**
     * 获取会员卡详情
     * GET /v1/memberships/:id
     */
    static async getMembership(id: number): Promise<Membership> {
        return await request({
            url: `/v1/memberships/${id}`,
            method: 'GET'
        })
    }
}

export default MembershipService
