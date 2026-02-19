import { getStableBarHeights } from '../../../../utils/responsive'
import {
  ComboManagementService,
  ComboSetWithDetailsResponse,
  DishManagementService,
  DishResponse
} from '../../../../api/dish'
import { logger } from '../../../../utils/logger'

type PricingMode = 'sum' | 'off_90' | 'off_80' | 'keep'

interface DishOption {
  id: number
  name: string
  price: number
  is_online: boolean
  checked: boolean
}

interface ComboEditOptions {
  id?: string
}

const PRICING_MODE_OPTIONS = [
  { label: '原价合计', value: 'sum' },
  { label: '9折', value: 'off_90' },
  { label: '8折', value: 'off_80' },
  { label: '保持当前价(仅编辑)', value: 'keep' }
]

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    submitting: false,
    isEdit: false,
    comboId: 0,
    existingName: '',
    existingPrice: 0,
    selectedDishIds: [] as number[],
    allDishes: [] as DishOption[],
    dishes: [] as DishOption[],
    showOnlineOnly: false,
    pricingModeOptions: PRICING_MODE_OPTIONS,
    pricingMode: 'off_90' as PricingMode,
    onlineChoice: 'online' as 'online' | 'offline',
    autoName: '精选套餐',
    originalTotal: 0,
    comboPricePreview: 0
  },

  onLoad(options: ComboEditOptions) {
    const { navBarHeight } = getStableBarHeights()
    const comboId = Number(options.id || 0)
    this.setData({
      navBarHeight,
      isEdit: comboId > 0,
      comboId,
      pricingMode: comboId > 0 ? 'keep' : 'off_90'
    })
    this.loadData()
  },

  async fetchAllDishes(): Promise<DishResponse[]> {
    const pageSize = 50
    let pageId = 1
    let totalCount = 0
    let allDishes: DishResponse[] = []

    do {
      const response = await DishManagementService.listDishes({
        page_id: pageId,
        page_size: pageSize
      })

      const pageDishes = Array.isArray(response?.dishes) ? response.dishes.filter((dish) => !!dish) : []
      allDishes = [...allDishes, ...pageDishes]
      totalCount = Number(response?.total_count || 0)
      pageId += 1

      if (!pageDishes.length) {
        break
      }
    } while (allDishes.length < totalCount)

    return allDishes
  },

  async loadData() {
    this.setData({ loading: true })
    try {
      const [allDishesResponse, comboRes] = await Promise.all([
        this.fetchAllDishes(),
        this.data.isEdit
          ? ComboManagementService.getComboDetail(this.data.comboId)
          : Promise.resolve(null as ComboSetWithDetailsResponse | null)
      ])

      const dishes = allDishesResponse.map((dish: DishResponse) => ({
        id: dish.id,
        name: dish.name,
        price: dish.price,
        is_online: dish.is_online,
        checked: false
      }))

      const selectedDishIds = comboRes?.dishes?.map((dish) => dish.dish_id) || []

      const selectedSet = new Set(selectedDishIds)
      const dishOptions = dishes.map((dish) => ({
        ...dish,
        checked: selectedSet.has(dish.id)
      }))

      this.setData({
        allDishes: dishOptions,
        selectedDishIds,
        existingName: comboRes?.name || '',
        existingPrice: comboRes?.combo_price || 0,
        onlineChoice: comboRes?.is_online === false ? 'offline' : 'online'
      })

      this.syncVisibleDishes()
      this.recomputePreview()
    } catch (err) {
      logger.error('Load combo edit data failed', err)
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  onDishCheckChange(e: WechatMiniprogram.CustomEvent) {
    const values = (e.detail.value || []) as string[]
    const selectedDishIds = values.map((value) => Number(value)).filter((id) => id > 0)
    const selectedSet = new Set(selectedDishIds)
    const allDishes = this.data.allDishes.map((dish) => ({
      ...dish,
      checked: selectedSet.has(dish.id)
    }))
    this.setData({ selectedDishIds, allDishes })
    this.syncVisibleDishes()
    this.recomputePreview()
  },

  onShowOnlineOnlyChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    this.setData({ showOnlineOnly: !!e.detail?.value })
    this.syncVisibleDishes()
  },

  onPricingModeChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ pricingMode: e.detail.value as PricingMode })
    this.recomputePreview()
  },

  onOnlineChoiceChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ onlineChoice: e.detail.value as 'online' | 'offline' })
  },

  calcOriginalTotal() {
    const dishPriceMap = new Map(this.data.allDishes.map((dish) => [dish.id, dish.price]))
    return this.data.selectedDishIds.reduce((sum, dishId) => sum + (dishPriceMap.get(dishId) || 0), 0)
  },

  calcComboPrice(originalTotal: number) {
    if (this.data.isEdit && this.data.pricingMode === 'keep') {
      return this.data.existingPrice
    }

    if (this.data.pricingMode === 'sum') {
      return originalTotal
    }
    if (this.data.pricingMode === 'off_90') {
      return Math.round(originalTotal * 0.9)
    }
    if (this.data.pricingMode === 'off_80') {
      return Math.round(originalTotal * 0.8)
    }

    return originalTotal
  },

  buildAutoName() {
    const dishMap = new Map(this.data.allDishes.map((dish) => [dish.id, dish.name]))
    const selectedNames = this.data.selectedDishIds
      .map((dishId) => dishMap.get(dishId) || '')
      .filter((name) => !!name)

    if (selectedNames.length === 0) {
      return this.data.isEdit ? this.data.existingName : '精选套餐'
    }

    if (selectedNames.length <= 2) {
      return `${selectedNames.join('+')}套餐`
    }

    return `${selectedNames.slice(0, 2).join('+')}等${selectedNames.length}款套餐`
  },

  async onSubmit() {
    if (this.data.submitting) return

    if (this.data.selectedDishIds.length === 0) {
      wx.showToast({ title: '请至少选择1个菜品', icon: 'none' })
      return
    }

    const originalTotal = this.calcOriginalTotal()
    if (originalTotal <= 0) {
      wx.showToast({ title: '套餐总价异常', icon: 'none' })
      return
    }

    const comboPrice = this.calcComboPrice(originalTotal)
    const name = this.buildAutoName()
    const isOnline = this.data.onlineChoice === 'online'

    this.setData({ submitting: true })
    try {
      if (this.data.isEdit) {
        await ComboManagementService.updateCombo(this.data.comboId, {
          name,
          combo_price: comboPrice,
          is_online: isOnline,
          dishes: this.data.selectedDishIds.map((dishId) => ({ dish_id: dishId, quantity: 1 }))
        })
      } else {
        await ComboManagementService.createCombo({
          name,
          combo_price: comboPrice,
          is_online: isOnline,
          dish_ids: this.data.selectedDishIds
        })
      }

      wx.showToast({ title: this.data.isEdit ? '套餐已更新' : '套餐已创建', icon: 'success' })
      setTimeout(() => {
        wx.navigateBack()
      }, 500)
    } catch (err) {
      logger.error('Submit combo failed', err)
      wx.showToast({ title: '保存失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  recomputePreview() {
    const originalTotal = this.calcOriginalTotal()
    const comboPricePreview = this.calcComboPrice(originalTotal)
    const autoName = this.buildAutoName()
    this.setData({ originalTotal, comboPricePreview, autoName })
  },

  syncVisibleDishes() {
    const dishes = this.data.showOnlineOnly
      ? this.data.allDishes.filter((dish) => dish.is_online || dish.checked)
      : this.data.allDishes
    this.setData({ dishes })
  }
})
