import RiderService from '@/api/rider'
import { locationService } from './location'

export function normalizeLocationError(error: unknown): Error {
    if (error && typeof error === 'object' && 'errMsg' in error) {
        const errMsg = (error as { errMsg?: string }).errMsg
        if (errMsg?.includes('auth deny') || errMsg?.includes('authorize')) {
            return new Error('请开启定位权限后重试')
        }
        if (errMsg?.includes('fail')) {
            return new Error('定位获取失败，请稍后重试')
        }
    }

    if (error instanceof Error) {
        return error
    }

    return new Error('定位获取失败，请稍后重试')
}

export async function syncRiderDeliveryLocation(deliveryId: number, source: string): Promise<void> {
    let location: { latitude: number, longitude: number }

    try {
        location = await locationService.getCurrentLocation()
    } catch (error) {
        throw normalizeLocationError(error)
    }

    await RiderService.updateLocation([
        {
            longitude: location.longitude,
            latitude: location.latitude,
            delivery_id: deliveryId,
            recorded_at: new Date().toISOString(),
            source
        }
    ])
}