import { getCurrentRegion } from '../api/location'
import { globalStore } from './global-store'

export async function resolveCurrentRegionId(): Promise<number> {
  const app = getApp<IAppOption>()
  const cachedRegionId = Number(app.globalData.currentRegion?.id || 0)
  if (cachedRegionId > 0) {
    return cachedRegionId
  }

  const latitude = Number(app.globalData.latitude || 0)
  const longitude = Number(app.globalData.longitude || 0)
  if (!latitude || !longitude) {
    throw new Error('当前定位区域未获取，请稍后重试')
  }

  const region = await getCurrentRegion({ latitude, longitude })
  const regionId = Number(region.region_id || 0)
  if (regionId <= 0) {
    throw new Error('当前定位区域未获取，请稍后重试')
  }

  const currentRegion = { id: regionId, name: region.region_name || '' }
  app.globalData.currentRegion = currentRegion
  globalStore.set('currentRegion', currentRegion)
  return regionId
}
