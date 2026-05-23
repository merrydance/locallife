import { request } from '../utils/request'

export interface CurrentRegionResponse {
    region_id: number
    region_name: string
    parent_id?: number
    parent_name?: string
}

export interface ReverseGeocodeResponse {
    address: string
    formatted_address: string
    province: string
    city: string
    district: string
    street: string
    street_number: string
}

export interface RegionSearchResult {
    id: number
    code: string
    name: string
    level: number
    parent_id?: number
    longitude?: string
    latitude?: string
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

export function searchRegions(query: string): Promise<RegionSearchResult[]> {
    return request<RegionSearchResult[]>({
        url: '/v1/regions/search',
        method: 'GET',
        data: { q: query.trim() }
    })
}

export function reverseGeocode(params: { latitude: number, longitude: number }): Promise<ReverseGeocodeResponse> {
    assertValidCoordinate(Number(params.latitude), Number(params.longitude))

    return request<ReverseGeocodeResponse>({
        url: '/v1/location/reverse-geocode',
        method: 'GET',
        data: {
            latitude: Number(params.latitude),
            longitude: Number(params.longitude)
        }
    })
}

export interface BicyclingRoutePoint {
    lat?: number
    lng?: number
    latitude?: number
    longitude?: number
}

export interface BicyclingDirectionData {
    distance?: number
    // LocalLife 后端已将腾讯 LBS 路线规划时长统一转换为秒。
    duration?: number
    points?: BicyclingRoutePoint[]
}

export interface BicyclingDirectionEnvelope {
    code: number
    message?: string
    data?: BicyclingDirectionData
}

export type BicyclingDirectionResponse = BicyclingDirectionData | BicyclingDirectionEnvelope

export function getBicyclingDirection(params: { from: string, to: string }): Promise<BicyclingDirectionResponse> {
    return request<BicyclingDirectionResponse>({
        url: '/v1/location/direction/bicycling',
        method: 'GET',
        data: params
    })
}

export interface ActiveCategory {
    id: number
    name: string
    merchant_count: number
}

/**
 * 获取当前区域内有商户覆盖的菜系品类，按商户数量降序。用于首页品类网格。
 */
export async function getActiveCategories(params: {
    user_latitude?: number
    user_longitude?: number
    region_id?: number
}): Promise<ActiveCategory[]> {
    const data: Record<string, unknown> = {}
    if (params.user_latitude !== undefined) data.user_latitude = params.user_latitude
    if (params.user_longitude !== undefined) data.user_longitude = params.user_longitude
    if (params.region_id !== undefined) data.region_id = params.region_id

    const response = await request<{ categories: ActiveCategory[] }>({
        url: '/v1/search/categories',
        method: 'GET',
        data,
        useCache: true,
        cacheTTL: 5 * 60 * 1000 // 5分钟缓存，品类变化不频繁
    })
    return response.categories || []
}
