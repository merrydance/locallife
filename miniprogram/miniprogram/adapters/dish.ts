import { Dish, DishResponse } from '../models/dish'
import { DishSummary as ApiDishSummary } from '../api/dish'

import { getPublicImageUrl } from '../utils/image'
import { formatPrice } from '../utils/util'


export class DishAdapter {
  /**
   * 将菜品响应DTO转换为视图模型 - 基于swagger api.dishResponse
   */
  static toViewModel(dto: DishResponse): Dish {
    return {
      id: dto.id,
      name: dto.name,
      imageUrl: getPublicImageUrl(dto.image_url),
      price: dto.price,
      priceDisplay: formatPrice(dto.price),
      shopName: '商户名称', // 需要从商户信息获取
      merchantId: dto.merchant_id,
      attributes: dto.ingredients?.map(ing => ing.name) || [],
      spicyLevel: 0, // 从tags中解析辣度
      salesBadge: '', // 菜品详情中没有销量信息
      ratingDisplay: '0.0', // 菜品详情中没有评分信息
      distance: '距离未知',
      deliveryTimeDisplay: DishAdapter.formatDeliveryTime(dto.prepare_time),
      deliveryFeeDisplay: '配送费待定',
      discountRule: '',
      tags: dto.tags?.map(tag => tag.name) || [],
      isPremade: dto.tags?.some(tag => tag.name.includes('预制')) || false,
      customization_groups: dto.customization_groups,
      member_price: dto.member_price,
      is_available: dto.is_available,
      prepare_time: dto.prepare_time
    }
  }

  /**
   * 将菜品摘要DTO转换为视图模型 - 基于swagger api.dishSummary (用于Feed流)
   */
  static fromSummaryDTO(dto: ApiDishSummary): Dish {
    return {
      id: dto.id,
      name: dto.name,
      imageUrl: getPublicImageUrl(dto.image_url),
      price: dto.price,
      priceDisplay: formatPrice(dto.price),
      shopName: dto.merchant_name || '未知商家',
      merchantId: dto.merchant_id,
      attributes: [], // 摘要数据中没有配料信息
      spicyLevel: 0, // 从tags中解析辣度
      salesBadge: DishAdapter.formatSales(dto.monthly_sales || 0),
      ratingDisplay: '0.0', // 摘要数据中没有评分
      distance: DishAdapter.formatDistance(dto.distance || 0),
      deliveryTimeDisplay: DishAdapter.formatDeliveryTimeSeconds(dto.estimated_delivery_time || 0),
      deliveryFeeDisplay: DishAdapter.formatDeliveryFee(dto.estimated_delivery_fee || 0),
      discountRule: '',
      tags: dto.tags || [],
      isPremade: dto.tags?.includes('预制') || false,
      merchantIsOpen: dto.merchant_is_open ?? true, // 商户营业状态，默认营业
      distance_meters: dto.distance || 0,
      member_price: dto.member_price,
      is_available: dto.is_available,
      repurchaseRate: dto.repurchase_rate,
      repurchaseRateDisplay: DishAdapter.formatRepurchaseRate(dto.repurchase_rate),
      estimated_delivery_time: dto.estimated_delivery_time
    }
  }


  // 兼容性：保留旧方法名
  static fromFeedDTO = DishAdapter.fromSummaryDTO

  static formatSales(sales: number): string {
    if (sales >= 1000) {
      return `月销${(sales / 1000).toFixed(1)}k`
    }
    return `月销${sales}`
  }

  static formatDistance(meters?: number): string {
    if (!meters || meters === 0) {
      return '距离未知'
    }
    if (meters < 1000) {
      return `${Math.round(meters)}米`
    }
    return `${(meters / 1000).toFixed(1)}公里`
  }

  static formatDeliveryTime(minutes?: number): string {
    if (!minutes || minutes === 0) {
      return '时间待定'
    }
    return `${minutes}分钟`
  }

  static formatDeliveryTimeSeconds(seconds?: number): string {
    if (!seconds || seconds === 0) {
      return '时间待定'
    }
    const minutes = Math.round(seconds / 60)
    if (minutes < 60) {
      return `约${minutes}分钟`
    }
    const hours = Math.floor(minutes / 60)
    const remainingMinutes = minutes % 60
    if (remainingMinutes === 0) {
      return `约${hours}小时`
    }
    return `约${hours}小时${remainingMinutes}分`
  }

  static formatDeliveryFee(fee: number): string {
    if (fee === 0) {
      return '免代取费'
    }
    // 添加"起"表示这是起步价，实际费用可能因订单金额而更高
    return `代取${(fee / 100).toFixed(0)}元起`
  }

  static formatDiscountRule(threshold: number, discountAmount?: number): string {
    if (threshold > 0) {
      const discount = discountAmount || Math.floor(threshold / 10 / 100)
      return `满${(threshold / 100).toFixed(0)}返${discount}元`
    }
    return ''
  }

  static formatRepurchaseRate(rate?: number): string {
    if (!rate || rate <= 0) return ''
    return `复购率${(rate * 100).toFixed(0)}%`
  }
}
