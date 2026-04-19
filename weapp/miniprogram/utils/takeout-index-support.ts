import ConsumerDiscoveryAdapter from '../adapters/consumer-discovery'
import { buildMerchantDisplayTags } from '../adapters/merchant-labels'
import { buildTakeoutCategoryGridItems, type TakeoutCategoryGridItem } from '../adapters/takeout-categories'
import { getActiveCategories } from '../api/location'
import {
  type MerchantSummary,
  type PublicDeliveryPromotion,
  type PublicDiscountRule,
  type PublicDish,
  type PublicMerchantDetail,
  type PublicVoucher
} from '../api/merchant'
import { logger } from './logger'
import { globalStore } from './global-store'
import { getPublicImageUrl } from './image'
import { formatPrice } from './util'

export const TAKEOUT_PAGE_SIZE = 10
export const TAKEOUT_DEFAULT_AVG_PREP_MINUTES = 15
export const TAKEOUT_FIRST_SCREEN_MERCHANT_COUNT = 3
export const TAKEOUT_HYDRATION_BATCH_SIZE = 2
export const TAKEOUT_PRIORITY_META_HYDRATION_DELAY_MS = 260
export const TAKEOUT_BACKGROUND_DISH_HYDRATION_DELAY_MS = 700
export const TAKEOUT_BACKGROUND_META_HYDRATION_DELAY_MS = 1200

export interface FeaturedDish {
  id: number
  name: string
  imageUrl: string
  priceDisplay: string
  price: number
  merchantId: number
  customization_groups?: unknown[]
}

export interface MerchantFeedViewModel {
  id: number
  name: string
  imageUrl: string
  isOpen: boolean
  isOrderingSuspended: boolean
  distance: string
  monthlySales: number
  deliveryFeeDisplay: string
  promoText: string
  subsidyText: string
  tags: string[]
  systemLabels: string[]
  displayTags: string[]
  featuredDishes: FeaturedDish[]
  dishesLoading: boolean
  avgPrepMinutes: number
  discountPromoText: string
  voucherText: string
  deliveryPromoText: string
  isNewStore: boolean
  hasOrdered: boolean
  detailLoading: boolean
  label?: string
}

export interface UserMessageError {
  userMessage?: string
}

export type SearchTimer = ReturnType<typeof setTimeout>

interface TakeoutPageLike {
  setData: (patch: Record<string, unknown>) => void
  loadData: () => void | Promise<void>
}

export function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export function chunkArray<T>(items: T[], size: number): T[][] {
  const chunks: T[][] = []
  for (let index = 0; index < items.length; index += size) {
    chunks.push(items.slice(index, index + size))
  }
  return chunks
}

export function deriveMerchantPromotions(tags: string[] = [], deliveryFee?: number) {
  const promoTag = tags.find((tag) => /促销|满减|折扣|优惠|券/.test(tag)) || ''
  let subsidyTag = tags.find((tag) => /补贴|免配送|免运费|运费减免|配送补贴/.test(tag)) || ''

  if (!subsidyTag && deliveryFee === 0) {
    subsidyTag = '运费补贴'
  }

  return {
    promoText: promoTag,
    subsidyText: subsidyTag
  }
}

export function buildTakeoutMerchantFeedItems(merchants: MerchantSummary[]) {
  return merchants.map((merchant) => {
    const merchantSummary = ConsumerDiscoveryAdapter.toMerchantSummaryViewModel(merchant)
    const isNewStore = merchant.created_at
      ? (Date.now() - new Date(merchant.created_at).getTime()) < 30 * 24 * 60 * 60 * 1000
      : false

    return {
      ...deriveMerchantPromotions(merchantSummary.tags, merchantSummary.deliveryFee),
      id: merchantSummary.id,
      name: merchantSummary.name,
      imageUrl: merchantSummary.imageUrl,
      isOpen: merchantSummary.isOpen,
      isOrderingSuspended: false,
      distance: merchantSummary.distanceDisplay,
      monthlySales: merchantSummary.monthlySales,
      deliveryFeeDisplay: merchantSummary.deliveryFeeDisplay,
      tags: merchantSummary.tags,
      systemLabels: merchantSummary.systemLabels,
      displayTags: merchantSummary.displayTags.slice(0, 3),
      featuredDishes: [],
      dishesLoading: true,
      avgPrepMinutes: TAKEOUT_DEFAULT_AVG_PREP_MINUTES,
      discountPromoText: '',
      voucherText: '',
      deliveryPromoText: '',
      isNewStore,
      hasOrdered: false,
      detailLoading: true,
      label: merchantSummary.label
    } satisfies MerchantFeedViewModel
  })
}

export async function buildTakeoutCategoriesState(activeCategoryId: string) {
  const app = getApp<IAppOption>()
  if (!app.globalData.latitude || !app.globalData.longitude) return null

  const { getToken } = require('./auth')
  if (!getToken()) return null

  const rawList = await getActiveCategories({
    user_latitude: app.globalData.latitude,
    user_longitude: app.globalData.longitude
  })

  const availableCategories = rawList.slice(0, 8)
  const hasRealCategories = availableCategories.length > 0
  const availableCategoryIds = new Set(availableCategories.map((category) => String(category.id)))
  const nextActiveCategoryId = activeCategoryId && availableCategoryIds.has(activeCategoryId)
    ? activeCategoryId
    : ''

  return {
    activeCategoryId: nextActiveCategoryId,
    cuisineCategories: hasRealCategories
      ? buildTakeoutCategoryGridItems(availableCategories, nextActiveCategoryId)
      : [] as TakeoutCategoryGridItem[]
  }
}

export async function tryTakeoutLoadData(
  page: TakeoutPageLike,
  retryCount: number,
  onLocationMissing: () => void
) {
  const maxTokenRetries = 20
  const retryInterval = 500
  const { getToken } = require('./auth')
  const app = getApp<IAppOption>()

  const token = getToken()
  const hasLocation = !!(app.globalData.latitude && app.globalData.longitude)

  if (!token) {
    if (retryCount >= maxTokenRetries) {
      logger.error('❌ 登录超时', { waitedTime: `${(retryCount * retryInterval) / 1000}秒` }, 'Takeout.tryLoadData')
      wx.showModal({
        title: '登录超时',
        content: '请检查网络连接后重试',
        confirmText: '重新加载',
        success: (res) => {
          if (res.confirm) {
            wx.reLaunch({ url: '/pages/takeout/index' })
          }
        }
      })
      return
    }

    if (retryCount === 0) {
      logger.info('等待登录...', undefined, 'Takeout.tryLoadData')
    }

    setTimeout(() => void tryTakeoutLoadData(page, retryCount + 1, onLocationMissing), retryInterval)
    return
  }

  if (!hasLocation) {
    logger.info('位置未授权，显示引导界面', undefined, 'Takeout.tryLoadData')
    onLocationMissing()
    return
  }

  logger.info('✅ Token 和位置都已准备好，开始加载数据', {
    tokenLength: token.length,
    locationName: app.globalData.location?.name || '未知'
  }, 'Takeout.tryLoadData')

  page.loadData()
}

export function showTakeoutLocationGuide(page: Pick<TakeoutPageLike, 'setData'>) {
  page.setData({
    needLocation: true,
    loading: false,
    address: '请先定位'
  })
  logger.info('显示位置引导提示', undefined, 'Takeout.showLocationGuide')
}

export function retryTakeoutLocation(page: Pick<TakeoutPageLike, 'setData'>) {
  const app = getApp<IAppOption>()
  page.setData({ address: '定位中...' })
  app.getLocationCoordinates()
}

export function openTakeoutLocationPicker(page: Pick<TakeoutPageLike, 'setData'>, loadData: () => void) {
  wx.chooseLocation({
    success: async (res) => {
      const app = getApp<IAppOption>()
      app.globalData.latitude = res.latitude
      app.globalData.longitude = res.longitude
      app.globalData.location = {
        name: res.name || res.address,
        address: res.address
      }

      globalStore.updateLocation(
        res.latitude,
        res.longitude,
        res.name || res.address,
        res.address
      )

      logger.info('用户手动选择位置', {
        latitude: res.latitude,
        longitude: res.longitude,
        name: res.name
      }, 'Takeout.openLocationPicker')

      page.setData({ address: res.name || res.address })
      loadData()
    },
    fail: (err) => {
      logger.warn('用户取消选择位置', err, 'Takeout.openLocationPicker')
      wx.showModal({
        title: '需要位置信息',
        content: '本地生活服务必须基于您的位置才能使用',
        confirmText: '重新选择',
        cancelText: '退出',
        success: (res) => {
          if (res.confirm) {
            openTakeoutLocationPicker(page, loadData)
          } else {
            wx.switchTab({ url: '/pages/user_center/index' })
          }
        }
      })
    }
  })
}

export function buildTakeoutFeaturedDishes(dishes: PublicDish[], merchantId: number): FeaturedDish[] {
  return dishes.slice(0, 3).map((dish) => ({
    id: dish.id,
    name: dish.name,
    imageUrl: getPublicImageUrl(dish.image_url || '') || '/assets/placeholder_food.png',
    priceDisplay: formatPrice(dish.price),
    price: dish.price,
    merchantId,
    customization_groups: dish.customization_groups || []
  }))
}

export function buildDiscountPromoText(discountRules?: PublicDiscountRule[]) {
  if (!discountRules || discountRules.length === 0) {
    return ''
  }

  const best = discountRules.reduce((current, next) => current.min_order_amount <= next.min_order_amount ? current : next)
  return `满${(best.min_order_amount / 100).toFixed(0)}减${(best.discount_amount / 100).toFixed(0)}`
}

export function buildVoucherText(vouchers?: PublicVoucher[]) {
  if (!vouchers || vouchers.length === 0) {
    return ''
  }

  const best = vouchers.reduce((current, next) => current.amount >= next.amount ? current : next)
  return `领券减${(best.amount / 100).toFixed(0)}元`
}

export function buildDeliveryPromoText(deliveryPromotions?: PublicDeliveryPromotion[]) {
  if (!deliveryPromotions || deliveryPromotions.length === 0) {
    return ''
  }

  const best = deliveryPromotions.reduce((current, next) => current.min_order_amount <= next.min_order_amount ? current : next)
  const threshold = best.min_order_amount
  return threshold === 0
    ? '免运费'
    : `满${(threshold / 100).toFixed(0)}免运费`
}

export function buildTakeoutMerchantMetaPatch(detail: PublicMerchantDetail, hasOrdered: boolean) {
  return {
    avgPrepMinutes: (detail.avg_prep_minutes && detail.avg_prep_minutes > 0)
      ? detail.avg_prep_minutes
      : TAKEOUT_DEFAULT_AVG_PREP_MINUTES,
    tags: detail.tags || [],
    systemLabels: detail.system_labels || [],
    displayTags: buildMerchantDisplayTags(detail.system_labels || [], detail.tags || [], 3),
    isOrderingSuspended: !!detail.is_ordering_suspended,
    discountPromoText: buildDiscountPromoText(detail.discount_rules),
    voucherText: buildVoucherText(detail.vouchers),
    deliveryPromoText: buildDeliveryPromoText(detail.delivery_promotions),
    hasOrdered
  }
}