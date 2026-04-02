import { getStableBarHeights } from '../../../../utils/responsive'
import {
  ComboDishInput,
  ComboManagementService,
  ComboSetWithDetailsResponse,
  DishManagementService,
  DishResponse,
  TagInfo,
  TagService
} from '../../../../api/dish'
import { getPublicImageUrl } from '../../../../utils/image'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type PricingMode = 'sum' | 'off_90' | 'off_80' | 'keep'

interface DishOption {
  id: number
  name: string
  price: number
  is_online: boolean
  image_url: string
  checked: boolean
  quantity: number
}

interface ComboEditOptions {
  id?: string
}

const getErrorMessage = getErrorUserMessage

const PRICING_MODE_OPTIONS = [
  { label: '原价合计', value: 'sum' },
  { label: '9折', value: 'off_90' },
  { label: '8折', value: 'off_80' },
  { label: '保持当前价(仅编辑)', value: 'keep' }
]

function normalizeDishQuantity(quantity?: number): number {
  if (!Number.isFinite(quantity) || !quantity) {
    return 1
  }

  const safeQuantity = Math.round(quantity)
  if (safeQuantity < 1) {
    return 1
  }
  if (safeQuantity > 99) {
    return 99
  }
  return safeQuantity
}

function buildSelectedComboDishes(dishes: DishOption[]): ComboDishInput[] {
  return dishes
    .filter((dish) => dish.checked)
    .map((dish) => ({
      dish_id: dish.id,
      quantity: normalizeDishQuantity(dish.quantity)
    }))
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    submitting: false,
    isEdit: false,
    comboId: 0,
    existingName: '',
    existingPrice: 0,
    comboName: '精选套餐',
    comboDescription: '',
    comboNameCustomized: false,
    availableTags: [] as TagInfo[],
    selectedTagIds: [] as number[],
    tagSubmitting: false,
    selectedDishIds: [] as number[],
    allDishes: [] as DishOption[],
    dishes: [] as DishOption[],
    showOnlineOnly: false,
    pricingModeOptions: PRICING_MODE_OPTIONS,
    pricingMode: 'off_90' as PricingMode,
    onlineChoice: 'online' as 'online' | 'offline',
    autoName: '精选套餐',
    originalTotal: 0,
    comboPricePreview: 0,
    selectedDishQuantityTotal: 0,
    selectedDishPreviews: [] as string[],
    dishEmptyDescription: '暂无可选菜品，请先创建菜品'
  },

  applyPersistedComboState(combo: Pick<ComboSetWithDetailsResponse, 'id' | 'name' | 'combo_price' | 'is_online'>, selectedDishes: ComboDishInput[]) {
    const quantityByDishId = new Map(selectedDishes.map((dish) => [dish.dish_id, normalizeDishQuantity(dish.quantity)]))
    const allDishes = this.data.allDishes.map((dish) => ({
      ...dish,
      checked: quantityByDishId.has(dish.id),
      quantity: quantityByDishId.get(dish.id) || dish.quantity || 1
    }))

    this.setData(
      {
        comboId: combo.id,
        isEdit: combo.id > 0,
        existingName: combo.name,
        existingPrice: combo.combo_price,
        comboName: combo.name,
        comboNameCustomized: true,
        onlineChoice: combo.is_online ? 'online' : 'offline',
        allDishes
      },
      () => {
        this.syncSelectedDishState()
        this.syncVisibleDishes()
        this.recomputePreview()
      }
    )
  },

  syncSelectedDishState() {
    const selectedDishIds = this.data.allDishes
      .filter((dish) => dish.checked)
      .map((dish) => dish.id)

    this.setData({ selectedDishIds })
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
      totalCount = Number(response?.total || 0)
      pageId += 1

      if (!pageDishes.length) {
        break
      }
    } while (allDishes.length < totalCount)

    return allDishes
  },

  async loadData() {
    this.setData({
      loading: true,
      initialError: false,
      initialErrorMessage: ''
    })
    try {
      const [allDishesResponse, comboRes, availableTags] = await Promise.all([
        this.fetchAllDishes(),
        this.data.isEdit
          ? ComboManagementService.getComboDetail(this.data.comboId)
          : Promise.resolve(null as ComboSetWithDetailsResponse | null),
        TagService.listTags('combo').catch(() => [] as TagInfo[])
      ])

      const dishes = allDishesResponse.map((dish: DishResponse) => ({
        id: dish.id,
        name: dish.name,
        price: dish.price,
        is_online: dish.is_online,
        image_url: getPublicImageUrl(dish.image_url || ''),
        checked: false,
        quantity: 1
      }))

      const quantityByDishId = new Map(
        (comboRes?.dishes || []).map((dish) => [dish.dish_id, normalizeDishQuantity(dish.quantity)])
      )
      const selectedDishIds = Array.from(quantityByDishId.keys())

      const selectedSet = new Set(selectedDishIds)
      const dishOptions = dishes.map((dish) => ({
        ...dish,
        checked: selectedSet.has(dish.id),
        quantity: quantityByDishId.get(dish.id) || 1
      }))

      this.setData({
        allDishes: dishOptions,
        selectedDishIds,
        existingName: comboRes?.name || '',
        existingPrice: comboRes?.combo_price || 0,
        comboName: comboRes?.name || this.data.comboName,
        comboDescription: comboRes?.description || '',
        comboNameCustomized: !!comboRes?.name,
        availableTags,
        selectedTagIds: Array.isArray(comboRes?.tags)
          ? comboRes.tags
            .map((tag) => Number(tag.id))
            .filter((id) => Number.isFinite(id) && id > 0)
          : [],
        onlineChoice: comboRes?.is_online === false ? 'offline' : 'online',
        initialError: false,
        initialErrorMessage: ''
      })

      this.syncVisibleDishes()
      this.recomputePreview()
    } catch (err) {
      logger.error('Load combo edit data failed', err)
      this.setData({
        initialError: true,
        initialErrorMessage: getErrorMessage(err, '套餐编辑页加载失败，请重试')
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onRetry() {
    this.loadData()
  },

  onDishCheckChange(e: WechatMiniprogram.CustomEvent) {
    const values = (e.detail.value || []) as string[]
    const selectedDishIds = values.map((value) => Number(value)).filter((id) => id > 0)
    const selectedSet = new Set(selectedDishIds)
    const allDishes = this.data.allDishes.map((dish) => ({
      ...dish,
      checked: selectedSet.has(dish.id),
      quantity: dish.quantity || 1
    }))
    this.setData({ allDishes }, () => {
      this.syncSelectedDishState()
      this.syncVisibleDishes()
      this.recomputePreview()
    })
  },

  onDishQuantityChange(
    e: WechatMiniprogram.CustomEvent<{ value: number }> & { currentTarget: { dataset: { id?: number } } }
  ) {
    const { id } = e.currentTarget.dataset
    if (!id) {
      return
    }

    const index = this.data.allDishes.findIndex((dish) => dish.id === id)
    if (index < 0 || !this.data.allDishes[index].checked) {
      return
    }

    const quantity = normalizeDishQuantity(e.detail?.value)
    this.setData(
      {
        [`allDishes[${index}].quantity`]: quantity
      },
      () => {
        this.syncVisibleDishes()
        this.recomputePreview()
      }
    )
  },

  onShowOnlineOnlyChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    this.setData({ showOnlineOnly: !!e.detail?.value })
    this.syncVisibleDishes()
  },

  onPricingModeChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ pricingMode: e.detail.value as PricingMode })
    this.recomputePreview()
  },

  onComboNameChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({
      comboName: (e.detail.value || '').replace(/^\s+/, ''),
      comboNameCustomized: true
    })
  },

  onComboDescriptionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({
      comboDescription: (e.detail.value || '').replace(/^\s+/, '')
    })
  },

  onTagChange(e: WechatMiniprogram.CustomEvent<{ value: string[] }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedTagIds = values
      .map((value) => Number(value))
      .filter((id) => Number.isFinite(id) && id > 0)

    this.setData({ selectedTagIds })
  },

  onCreateTag() {
    if (this.data.tagSubmitting) {
      return
    }

    wx.showModal({
      title: '新增套餐标签',
      editable: true,
      placeholderText: '请输入标签名称',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        const name = (res.content || '').trim()
        if (!name) {
          wx.showToast({ title: '标签名称不能为空', icon: 'none' })
          return
        }

        this.setData({ tagSubmitting: true })
        try {
          const created = await TagService.createTag({ name, type: 'combo' })
          const availableTags = this.data.availableTags.some((tag) => tag.id === created.id)
            ? this.data.availableTags
            : [...this.data.availableTags, created]
          const selectedTagIds = this.data.selectedTagIds.includes(created.id)
            ? this.data.selectedTagIds
            : [...this.data.selectedTagIds, created.id]

          this.setData({
            availableTags,
            selectedTagIds
          })
        } catch (err) {
          logger.error('Create combo tag failed', err)
          wx.showToast({ title: getErrorMessage(err, '新增标签失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ tagSubmitting: false })
        }
      }
    })
  },

  onOnlineChoiceChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ onlineChoice: e.detail.value as 'online' | 'offline' })
  },

  calcOriginalTotal() {
    return this.data.allDishes.reduce((sum, dish) => {
      if (!dish.checked) {
        return sum
      }
      return sum + dish.price * normalizeDishQuantity(dish.quantity)
    }, 0)
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
    const selectedNames = this.data.allDishes
      .filter((dish) => dish.checked)
      .map((dish) => {
        const quantity = normalizeDishQuantity(dish.quantity)
        return quantity > 1 ? `${dish.name}x${quantity}` : dish.name
      })
      .filter((name) => !!name)

    if (selectedNames.length === 0) {
      return this.data.isEdit ? this.data.existingName : '精选套餐'
    }

    if (selectedNames.length <= 2) {
      return `${selectedNames.join('+')}套餐`
    }

    return `${selectedNames.slice(0, 2).join('+')}等${selectedNames.length}款套餐`
  },

  buildComboName(autoName: string) {
    const comboName = this.data.comboName.trim()
    return comboName || autoName
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
  const autoName = this.buildAutoName()
  const name = this.buildComboName(autoName)
  const description = this.data.comboDescription.trim()
    const isOnline = this.data.onlineChoice === 'online'
    const selectedDishes = buildSelectedComboDishes(this.data.allDishes)

    this.setData({ submitting: true })
    try {
      let savedCombo: Pick<ComboSetWithDetailsResponse, 'id' | 'name' | 'combo_price' | 'is_online'>
      if (this.data.isEdit) {
        savedCombo = await ComboManagementService.updateCombo(this.data.comboId, {
          name,
          description,
          combo_price: comboPrice,
          is_online: isOnline,
          dishes: selectedDishes,
          tag_ids: this.data.selectedTagIds
        })
      } else {
        savedCombo = await ComboManagementService.createCombo({
          name,
          description,
          original_price: originalTotal,
          combo_price: comboPrice,
          is_online: isOnline,
          dishes: selectedDishes,
          tag_ids: this.data.selectedTagIds
        })
      }

      this.applyPersistedComboState(savedCombo, selectedDishes)

      const pages = getCurrentPages()
      const prevPage = pages[pages.length - 2] as { loadCombos?: (reset?: boolean) => void } | undefined
      if (prevPage?.loadCombos) {
        prevPage.loadCombos(true)
      }

      wx.navigateBack()
    } catch (err) {
      logger.error('Submit combo failed', err)
      wx.showToast({ title: getErrorMessage(err, '保存失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  recomputePreview() {
    const originalTotal = this.calcOriginalTotal()
    const comboPricePreview = this.calcComboPrice(originalTotal)
    const autoName = this.buildAutoName()
    const selectedDishQuantityTotal = this.data.allDishes
      .filter((dish) => dish.checked)
      .reduce((sum, dish) => sum + normalizeDishQuantity(dish.quantity), 0)
    const selectedDishPreviews = this.data.allDishes
      .filter((dish) => dish.checked && dish.image_url)
      .map((dish) => dish.image_url)
      .slice(0, 4)

    const updates: WechatMiniprogram.Page.DataOption = {
      originalTotal,
      comboPricePreview,
      autoName,
      selectedDishQuantityTotal,
      selectedDishPreviews
    }

    if (!this.data.comboNameCustomized) {
      updates.comboName = autoName
    }

    this.setData(updates)
  },

  syncVisibleDishes() {
    const dishes = this.data.showOnlineOnly
      ? this.data.allDishes.filter((dish) => dish.is_online || dish.checked)
      : this.data.allDishes
    const dishEmptyDescription = this.data.allDishes.length === 0
      ? '暂无可选菜品，请先创建菜品'
      : this.data.showOnlineOnly
        ? '当前没有已上架菜品，可先关闭筛选或先去上架菜品'
        : '暂无可选菜品，请先创建菜品'
    this.setData({ dishes, dishEmptyDescription })
  }
})
