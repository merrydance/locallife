/**
 * 套餐详情页面
 */

import { ComboManagementService, ComboSetWithDetailsResponse } from '../../../api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { formatPriceNoSymbol } from '../../../utils/util'
import { tracker, EventType } from '../../../utils/tracker'

type ValueEvent<T> = WechatMiniprogram.CustomEvent<{ value: T }>

function getErrorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'userMessage' in error) {
    const userMessage = (error as { userMessage?: string }).userMessage
    if (userMessage) return userMessage
  }
  return fallback
}

type ExtraInfo = {
  merchantName?: string
  monthSales?: number
  distanceMeters?: number
  estimatedDeliveryTime?: number
}

type ComboViewModel = {
  id: number
  name: string
  description: string
  image_url: string
  combo_price: number
  comboPriceDisplay: string
  original_price: number
  originalPriceDisplay: string
  savings_percent: number
  dishes: Array<{
    dish_id: number
    dish_name: string
    dish_price?: number
    dishPriceDisplay?: string
    dish_image_url?: string
    quantity: number
  }>
  tags: string[]
  // 额外展示字段
  merchant_name: string
  merchant_id: number
  month_sales: number
  distance_meters: number
  distance_km_display: string
  estimated_delivery_time: number
  estimated_delivery_time_display: string
  dish_images: string[] // 用于拼图展示
  selectedTags: Record<string, string> // 用于模拟规格选择
  is_open: boolean
  status_display: string
}

Page({
  data: {
    comboId: '',
    merchantId: '',
    combo: null as ComboViewModel | null,
    quantity: 1,
    loading: true,
    isError: false,
    errorMsg: '',
    totalPrice: 0,
    totalPriceDisplay: '0.00',
    extraInfo: {} as ExtraInfo,
    navBarHeight: 88, // 占位，会被组件更新
    remark: '',
    quickRemarks: ['少辣', '不要葱', '不要香菜', '少放盐', '多加饭']
  },

  onLoad(options: Record<string, string>) {
    const comboId = options.id
    const merchantId = options.merchant_id || options.shop_id || options.merchantId || ''
    const merchantName = decodeURIComponent(options.merchant_name || options.shop_name || options.merchantName || '')
    const monthSales = parseInt(options.month_sales || '0')
    const distanceMeters = parseInt(options.distance || '0')
    const estimatedDeliveryTime = parseInt(options.estimated_delivery_time || options.delivery_time || '0')

    if (!comboId) {
      wx.showToast({ title: '套餐ID缺失', icon: 'error' })
      setTimeout(() => wx.navigateBack(), 1500)
      return
    }

    this.setData({
      comboId,
      merchantId,
      extraInfo: {
        merchantName,
        monthSales,
        distanceMeters,
        estimatedDeliveryTime
      }
    })
    this.loadComboDetail()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadComboDetail() {
    this.setData({ loading: true, isError: false })

    try {
      const comboId = parseInt(this.data.comboId)
      const comboData: ComboSetWithDetailsResponse = await ComboManagementService.getPublicComboDetail(comboId)

      if (!comboData) {
        this.setData({ 
          loading: false, 
          isError: true, 
          errorMsg: '该套餐已售罄或下架' 
        })
        return
      }

      // ... (构建逻辑保持不变)
      const extraInfo = this.data.extraInfo
      const imageUrl = getPublicImageUrl(comboData.image_url || '')
      
      let originalPrice = comboData.original_price || 0
      if (!originalPrice && comboData.dishes) {
        originalPrice = comboData.dishes.reduce((sum, d) => sum + (d.dish_price || 0) * (d.quantity || 1), 0)
      }

      const savingsPercent = originalPrice > 0 
        ? Math.round(((originalPrice - comboData.combo_price) / originalPrice) * 100)
        : 0

      const distanceKmDisplay = extraInfo.distanceMeters && extraInfo.distanceMeters > 0
        ? `${(extraInfo.distanceMeters / 1000).toFixed(1)}km`
        : ''
      const estimatedDeliveryDisplay = extraInfo.estimatedDeliveryTime && extraInfo.estimatedDeliveryTime > 0
        ? `预计${extraInfo.estimatedDeliveryTime}分钟送达`
        : ''

      const dishImages = (comboData.dish_images && comboData.dish_images.length > 0)
        ? comboData.dish_images.map((url) => getPublicImageUrl(url))
        : (comboData.dishes || []).map((d) => getPublicImageUrl(d.dish_image_url || '')).filter(Boolean)
      const selectedTags: Record<string, string> = {}

      const combo: ComboViewModel = {
        id: comboData.id,
        name: comboData.name,
        description: comboData.description || '',
        image_url: imageUrl,
        combo_price: comboData.combo_price,
        comboPriceDisplay: formatPriceNoSymbol(comboData.combo_price),
        original_price: originalPrice,
        originalPriceDisplay: formatPriceNoSymbol(originalPrice),
        savings_percent: savingsPercent,
        dishes: (comboData.dishes || []).map((d) => ({
          ...d,
          quantity: d.quantity || 1,
          dish_image_url: getPublicImageUrl(d.dish_image_url || ''),
          dishPriceDisplay: d.dish_price ? formatPriceNoSymbol(d.dish_price) : undefined
        })),
        tags: comboData.tags?.map((t) => t.name) || [],
        merchant_name: extraInfo.merchantName || '商家',
        merchant_id: comboData.merchant_id || Number(this.data.merchantId) || 0,
        month_sales: extraInfo.monthSales || 0,
        distance_meters: extraInfo.distanceMeters || 0,
        distance_km_display: distanceKmDisplay,
        estimated_delivery_time: extraInfo.estimatedDeliveryTime || 0,
        estimated_delivery_time_display: estimatedDeliveryDisplay,
        dish_images: dishImages,
        selectedTags,
        is_open: comboData.is_open ?? true,
        status_display: comboData.is_open === false ? '商户休息中' : ''
      }

      this.setData({
        combo,
        loading: false,
        totalPrice: combo.combo_price,
        totalPriceDisplay: combo.comboPriceDisplay
      })

      // 埋点
      tracker.log(EventType.VIEW_DISH, String(combo.id), {
        is_combo: true,
        shop_id: combo.merchant_id,
        price: combo.combo_price
      })

    } catch (error: unknown) {
      console.error('加载套餐详情失败:', error)
      this.setData({ 
        loading: false, 
        isError: true, 
        errorMsg: getErrorMessage(error, '加载详情失败') 
      })
    }
  },

  onQuantityChange(e: ValueEvent<number>) {
    const quantity = e.detail.value
    const totalPrice = (this.data.combo?.combo_price || 0) * quantity
    this.setData({ 
      quantity,
      totalPrice,
      totalPriceDisplay: formatPriceNoSymbol(totalPrice)
    })
  },

  onTagTap(e: WechatMiniprogram.TouchEvent) {
    const tag = (e.currentTarget.dataset as { tag?: string }).tag
    const { combo } = this.data
    if (!combo || !tag) return

    const selectedTags = { ...combo.selectedTags }
    if (selectedTags[tag]) {
      delete selectedTags[tag]
    } else {
      selectedTags[tag] = tag
    }

    this.setData({
      'combo.selectedTags': selectedTags
    })
  },

  async onAddToCart() {
    const { combo, quantity, totalPrice } = this.data
    if (!combo) return

    const CartService = require('../../../services/cart').default
    
    // 构造规格描述字符串
    const tagsDesc = Object.values(combo.selectedTags).join(',')
    const finalRemark = this.data.remark ? `备注: ${this.data.remark}` : ''
    const specDesc = [tagsDesc, finalRemark].filter(Boolean).join('; ')
    
    console.log('[ComboDetail] Adding item to cart:', {
      merchantId: combo.merchant_id,
      comboId: combo.id,
      quantity,
      specs: specDesc
    })

    const success = await CartService.addItem({
      merchantId: combo.merchant_id,
      comboId: combo.id,
      quantity,
      customizations: specDesc ? { selected_options: specDesc } : undefined
    })

    if (!success) return

    tracker.log(EventType.ADD_CART, String(combo.id), {
      is_combo: true,
      shop_id: combo.merchant_id, // Keep shop_id for tracker for now, as it might be an external system requirement
      quantity,
      price: totalPrice,
      specs: specDesc
    })

    wx.showToast({ title: '已加入购物车', icon: 'success' })
  },

  onBuyNow() {
    this.onAddToCart()
    wx.navigateTo({ url: '/pages/takeout/cart/index' })
  },

  onShopTap() {
    const { combo, merchantId } = this.data
    const targetId = combo?.merchant_id || Number(merchantId)
    
    console.log('[ComboDetail] onShopTap called', { combo, merchantId, targetId })
    
    if (targetId) {
      const url = `/pages/takeout/restaurant-detail/index?id=${targetId}`
      console.log('[ComboDetail] Navigating to:', url)
      wx.navigateTo({ 
        url,
        fail: (err) => {
          console.error('[ComboDetail] Navigation failed:', err)
          wx.showToast({ title: '跳转失败', icon: 'none' })
        }
      })
    } else {
      console.warn('[ComboDetail] Cannot navigate: merchant_id is missing')
      wx.showToast({ title: '商家信息加载中', icon: 'none' })
    }
  },

  onDishTap(e: WechatMiniprogram.TouchEvent) {
    const dishId = (e.currentTarget.dataset as { id?: number }).id
    const merchantId = this.data.combo?.merchant_id || this.data.merchantId
    if (dishId && merchantId) {
      wx.navigateTo({ url: `/pages/takeout/dish-detail/index?id=${dishId}&merchant_id=${merchantId}` })
    }
  },

  onRemarkChange(e: ValueEvent<string>) {
    this.setData({ remark: e.detail.value })
  },

  onQuickRemarkTap(e: WechatMiniprogram.TouchEvent) {
    const item = (e.currentTarget.dataset as { item?: string }).item
    if (!item) return
    let { remark } = this.data
    if (remark) {
      if (!remark.includes(item)) {
        remark = `${remark}, ${item}`
      }
    } else {
      remark = item
    }
    this.setData({ remark })
  },

  onBack() {
    wx.navigateBack()
  },

  onShare() {
    const { combo } = this.data
    if (!combo) return
    
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })
  },

  onShareAppMessage() {
    const { combo } = this.data
    if (!combo) return {}
    
    return {
      title: `${combo.name} - 只要 ¥${combo.comboPriceDisplay}`,
      path: `/pages/takeout/combo-detail/index?id=${combo.id}&merchant_id=${combo.merchant_id}&merchant_name=${encodeURIComponent(combo.merchant_name)}`,
      imageUrl: combo.image_url
    }
  }
})
