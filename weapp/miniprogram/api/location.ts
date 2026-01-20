import { request } from '../utils/request'

export interface BicyclingDirectionResponse {
    code: number
    message?: string
    data?: {
        distance?: number
        duration?: number
    }
}

export function getBicyclingDirection(params: { from: string; to: string }): Promise<BicyclingDirectionResponse> {
    return request({
        url: '/v1/location/direction/bicycling',
        method: 'GET',
        data: params
    })
}
