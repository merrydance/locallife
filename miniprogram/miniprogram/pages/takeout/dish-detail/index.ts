/**
 * 菜品详情页面
 * 使用真实后端API
 */

import { tracker, EventType } from '../../../utils/tracker'
import { DishManagementService, DishResponse } from '../../../api/dish'
import { getMerchantReviews } from '../../../api/personal'
import { getPublicImageUrl } from '../../../utils/image'
import { formatPriceNoSymbol } from '../../../utils/util'

Page({
  data: {
    dishId: '',
    merchantId: '',
    dish: null as any,
    selectedSpecs: {} as Record<string, string>,
    quantity: 1,
    navBarHeight: 88,
    currentImageIndex: 0,
    loading: true,
    totalPrice: 0,
    totalPriceDisplay: '0.00'
  },

  onLoad(options: any) {
    const dishId = options.id
    const merchantId = options.merchant_id || ''
    // 从列表页传递过来的额外信息
    const shopName = decodeURIComponent(options.shop_name || '')
    const monthSales = parseInt(options.month_sales || '0')
    const distanceMeters = parseInt(options.distance || '0')
    const estimatedDeliveryTime = parseInt(options.estimated_delivery_time || '0')

    if (!dishId) {
      wx.showToast({ title: '菜品ID缺失', icon: 'error' })
      setTimeout(() => wx.navigateBack(), 1500)
      return
    }

    this.setData({
      dishId,
      merchantId,
      extraInfo: { shopName, monthSales, distanceMeters, estimatedDeliveryTime }
    })
    this.loadDishDetail()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadDishDetail() {
    this.setData({ loading: true })

    try {
      const dishId = parseInt(this.data.dishId)

      // 获取菜品详情
      const dishData: DishResponse = await DishManagementService.getDishDetail(dishId)

      if (!dishData) {
        wx.showToast({ title: '菜品不存在', icon: 'error' })
        this.setData({ loading: false })
        return
      }

      // 加载评价（如果有商户ID）
      let reviews: any[] = []
      if (dishData.merchant_id) {
        try {
          const reviewsResult = await getMerchantReviews(dishData.merchant_id, {
            page_id: 1,
            page_size: 5
          })
          reviews = (reviewsResult.reviews || []).map(r => ({
            user_name: '用户' + r.user_id,
            content: r.content,
            images: r.images || [],
            created_at: r.created_at
          }))
        } catch (e) {
          console.warn('加载评价失败:', e)
        }
      }

      // 从 URL 参数获取额外信息
      const extraInfo = (this.data as any).extraInfo || {}
      const imageUrl = getPublicImageUrl(dishData.image_url)

      // 构建菜品视图模型
      const dish = {
        id: dishData.id,
        name: dishData.name,
        shop_name: extraInfo.shopName || '商家',
        shop_id: dishData.merchant_id,
        merchant_id: dishData.merchant_id,
        images: imageUrl ? [imageUrl] : [],
        image_url: imageUrl,
        price: dishData.price,
        priceDisplay: formatPriceNoSymbol(dishData.price || 0),
        original_price: dishData.price,
        originalPriceDisplay: formatPriceNoSymbol(dishData.price || 0),
        member_price: dishData.member_price,
        memberPriceDisplay: dishData.member_price ? formatPriceNoSymbol(dishData.member_price) : null,
        description: dishData.description || '',
        is_available: dishData.is_available,
        is_online: dishData.is_online,
        prepare_time: dishData.prepare_time,
        spec_groups: this.convertCustomizationGroups(dishData.customization_groups),
        reviews,
        tags: dishData.tags?.map(t => t.name) || [],
        ingredients: dishData.ingredients || [],
        // 额外展示字段（从列表页传递）
        month_sales: extraInfo.monthSales || 0,
        distance_meters: extraInfo.distanceMeters || 0,
        estimated_delivery_time: extraInfo.estimatedDeliveryTime || Math.round((dishData.prepare_time || 10) + 15) // 制作时间+配送时间
      }

      // 初始化规格选择
      const selectedSpecs: Record<string, string> = {}
      if (dish.spec_groups) {
        dish.spec_groups.forEach((group: any) => {
          if (group.specs && group.specs.length > 0) {
            selectedSpecs[group.id] = group.specs[0].id
          }
        })
      }

      this.setData({
        dish,
        selectedSpecs,
        loading: false
      })

      this.updateTotalPrice()

      // 埋点
      tracker.log(EventType.VIEW_DISH, String(dish.id), {
        shop_id: dish.shop_id,
        price: dish.price,
        tags: dish.tags
      })

    } catch (error) {
      console.error('加载菜品详情失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  // 转换定制化分组为规格组格式
  convertCustomizationGroups(groups: any[] | undefined): any[] {
    if (!groups || groups.length === 0) return []

    return groups.map(group => ({
      id: group.id.toString(),
      name: group.name,
      is_required: group.is_required,
      specs: (group.options || []).map((opt: any) => ({
        id: opt.id.toString(),
        name: opt.tag_name,
        price_diff: opt.extra_price || 0,
        priceDiffDisplay: opt.extra_price ? formatPriceNoSymbol(opt.extra_price) : null
      }))
    }))
  },

  onImageChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ currentImageIndex: e.detail.current })
  },

  onSpecTap(e: WechatMiniprogram.CustomEvent) {
    const { groupId, specId } = e.currentTarget.dataset
    const { selectedSpecs } = this.data

    if (selectedSpecs[groupId] === specId) return

    this.setData({
      [`selectedSpecs.${groupId}`]: specId
    })
    this.updateTotalPrice()
  },

  updateTotalPrice() {
    const { dish, selectedSpecs } = this.data
    if (!dish) return

    let totalPrice = dish.price

    if (dish.spec_groups) {
      dish.spec_groups.forEach((group: any) => {
        const selectedSpecId = selectedSpecs[group.id]
        const spec = group.specs?.find((s: any) => s.id === selectedSpecId)
        if (spec) {
          totalPrice += spec.price_diff
        }
      })
    }

    this.setData({
      totalPrice,
      totalPriceDisplay: formatPriceNoSymbol(totalPrice)
    })
  },

  onQuantityChange(e: WechatMiniprogram.CustomEvent) {
    const { type } = e.currentTarget.dataset
    let { quantity } = this.data

    if (type === 'minus' && quantity > 1) {
      quantity--
    } else if (type === 'plus') {
      quantity++
    }

    this.setData({ quantity })
  },

  async onAddToCart() {
    const { dish, selectedSpecs, quantity, totalPrice } = this.data
    if (!dish) return

    // 构建规格描述
    const specNames: string[] = []
    if (dish.spec_groups) {
      dish.spec_groups.forEach((group: any) => {
        const selectedSpecId = selectedSpecs[group.id]
        const spec = group.specs?.find((s: any) => s.id === selectedSpecId)
        if (spec) {
          specNames.push(spec.name)
        }
      })
    }
    const specDesc = specNames.length > 0 ? `(${specNames.join('/')})` : ''

    const CartService = require('../../../services/cart').default
    const success = await CartService.addItem({
      merchantId: dish.shop_id || dish.merchant_id,
      dishId: dish.id,
      dishName: `${dish.name}${specDesc}`,
      shopName: dish.shop_name,
      imageUrl: dish.images?.[0] || dish.image_url,
      price: totalPrice,
      priceDisplay: `¥${(totalPrice / 100).toFixed(2)}`,
      quantity
    })

    if (!success) {
      return
    }

    tracker.log(EventType.ADD_CART, dish.id, {
      shop_id: dish.shop_id,
      quantity,
      price: totalPrice,
      tags: dish.tags
    })

    wx.showToast({ title: '已加入购物车', icon: 'success' })
  },

  onBuyNow() {
    this.onAddToCart()
    wx.navigateTo({ url: '/pages/takeout/cart/index' })
  },

  onShopTap() {
    const { dish } = this.data
    if (dish && dish.shop_id) {
      wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${dish.shop_id}` })
    }
  }
})
