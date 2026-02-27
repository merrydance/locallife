import { request } from '../utils/request'

export interface CurrentRegionResponse {
    region_id: number
    region_name: string
    parent_id?: number
    parent_name?: string
}

function assertValidCoordinate(latitude: number, longitude: number): void {
    if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
        throw new Error('invalid coordinates')
    }
}

export function getCurrentRegion(params: { latitude: number, longitude: number }): Promise<CurrentRegionResponse> {
    assertValidCoordinate(Number(params.latitude), Number(params.longitude))

    return request<CurrentRegionResponse>({
        url: '/v1/location/current-region',
        method: 'GET',
        data: {
            latitude: Number(params.latitude),
            longitude: Number(params.longitude)
        }
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
