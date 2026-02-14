import { request } from '../utils/request'

export interface CurrentRegionResponse {
    region_id: number
    region_name: string
    parent_id?: number
    parent_name?: string
}

export function getCurrentRegion(params: { latitude: number, longitude: number }): Promise<CurrentRegionResponse> {
    return request<CurrentRegionResponse>({
        url: '/v1/location/current-region',
        method: 'GET',
        data: params
    })
}

export interface BicyclingDirectionResponse {
    code: number
    message?: string
    data?: {
        distance?: number
        duration?: number
    }
}

export function getBicyclingDirection(params: { from: string, to: string }): Promise<BicyclingDirectionResponse> {
    return request({
        url: '/v1/location/direction/bicycling',
        method: 'GET',
        data: params
    })
}
