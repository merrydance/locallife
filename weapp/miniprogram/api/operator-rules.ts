import { request } from '../utils/request'

export type OperatorRuleCategory = 'delivery' | 'weather' | 'timeslot'
export type OperatorRuleAction = 'edit' | 'navigate_peak'

export interface OperatorRuleItem {
    id: string
    name: string
    key: string
    value: string
    unit: string
    desc: string
    editable: boolean
    category: string
    action?: OperatorRuleAction
}

export interface ListOperatorRulesParams extends Record<string, unknown> {
    region_id?: number
}

export interface ListOperatorRulesResponse {
    rules: OperatorRuleItem[]
}

export interface UpdateOperatorRuleRequest extends Record<string, unknown> {
    value: string
}

export class OperatorRulesService {
    async listRules(params?: ListOperatorRulesParams): Promise<ListOperatorRulesResponse> {
        return request({
            url: '/v1/operator/rules',
            method: 'GET',
            data: params
        })
    }

    async updateRule(key: string, data: UpdateOperatorRuleRequest, regionId?: number): Promise<void> {
        const regionQuery = regionId ? `?region_id=${regionId}` : ''
        return request({
            url: `/v1/operator/rules/${key}${regionQuery}`,
            method: 'PATCH',
            data
        })
    }
}

export const operatorRulesService = new OperatorRulesService()

export class OperatorRulesAdapter {
    static normalizeCategory(category?: string): OperatorRuleCategory {
        if (category === 'weather' || category === 'timeslot') {
            return category
        }

        return 'delivery'
    }

    static getCategoryIcon(category: OperatorRuleCategory): string {
        if (category === 'weather') {
            return 'cloud'
        }

        if (category === 'timeslot') {
            return 'time'
        }

        return 'chart'
    }
}