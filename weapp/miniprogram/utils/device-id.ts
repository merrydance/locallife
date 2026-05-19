import { logger } from './logger'

const STORAGE_KEY = 'device_id'

export function getDeviceId(): string {
  try {
    const deviceId = wx.getStorageSync(STORAGE_KEY)
    if (deviceId) {
      return deviceId
    }
  } catch (err) {
    logger.warn('读取缓存的device_id失败', err, 'getDeviceId')
  }

  const deviceId = `mp_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`

  try {
    wx.setStorageSync(STORAGE_KEY, deviceId)
    logger.info('生成新的device_id', { deviceId }, 'getDeviceId')
  } catch (err) {
    logger.warn('保存device_id失败', err, 'getDeviceId')
  }

  return deviceId
}
