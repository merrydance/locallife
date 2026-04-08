import { getStableBarHeights } from '../../../../utils/responsive'
import {
  ComboDishInput,
  ComboManagementService,
  ComboSetWithDetailsResponse,
  CustomizationGroup,
  DishManagementService,
  DishResponse,
  TagInfo,
  TagService
} from '../../../../api/dish'
import { getPublicImageUrl } from '../../../../utils/image'
import { logger } from '../../../../utils/logger'
import { settleAll } from '../../../../utils/promise'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type CreatePopupMode = 'tag' | ''

interface DishOption {
  id: number
  name: string
  price: number
  is_available: boolean
  is_online: boolean
  image_url: string
  checked: boolean
  quantity: number
  customization_groups: CustomizationGroup[]
  customization_error_message: string
  customizations: Record<string, number | string>
  customization_summary: string
  customization_extra_price: number
}

interface ComboEditOptions {
  id?: string
}

interface FormInputDetail {
  value: string
}

type SelectedSpecState = Record<string, string>

interface SelectedComboDishState {
  quantity: number
  customizations: Record<string, string>
  customizationSummary: string
  customizationExtraPrice: number
}

const getErrorMessage = getErrorUserMessage

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
    .map((dish) => {
      const comboDish: ComboDishInput = {
        dish_id: dish.id,
        quantity: normalizeDishQuantity(dish.quantity)
      }

      if (Object.keys(dish.customizations || {}).length > 0) {
        comboDish.customizations = dish.customizations
      }

      return comboDish
    })
}

function normalizeComboCustomizations(customizations?: Record<string, unknown> | null): Record<string, string> {
  const normalized: Record<string, string> = {}

  if (!customizations || typeof customizations !== 'object') {
    return normalized
  }

  Object.entries(customizations).forEach(([key, value]) => {
    if (key === 'meta_specs') {
      if (typeof value === 'string' && value.trim()) {
        normalized[key] = value.trim()
      }
      return
    }

    if (typeof value === 'number' || typeof value === 'string') {
      normalized[key] = String(value)
    }
  })

  return normalized
}

function buildSelectedSpecState(groups: CustomizationGroup[], customizations: Record<string, number | string>) {
  const selectedSpecs: SelectedSpecState = {}
  let hasInvalidSelection = false

  groups.forEach((group) => {
    const rawValue = customizations[String(group.id)]
    const existingSpecId = rawValue ? String(rawValue) : ''
    const matchedOption = (group.options || []).find((option) => String(option.id) === existingSpecId)

    if (matchedOption) {
      selectedSpecs[String(group.id)] = String(matchedOption.id)
      return
    }

    if (existingSpecId) {
      hasInvalidSelection = true
    }

    if (group.is_required && Array.isArray(group.options) && group.options.length > 0) {
      selectedSpecs[String(group.id)] = String(group.options[0].id)
    }
  })

  return { selectedSpecs, hasInvalidSelection }
}

function buildDishCustomizationPayload(groups: CustomizationGroup[], selectedSpecs: SelectedSpecState) {
  const customizations: Record<string, number | string> = {}
  const specNames: string[] = []
  let extraPrice = 0

  groups.forEach((group) => {
    const selectedSpecId = selectedSpecs[String(group.id)]
    if (!selectedSpecId) {
      return
    }

    const option = (group.options || []).find((candidate) => String(candidate.id) === selectedSpecId)
    if (!option) {
      return
    }

    customizations[String(group.id)] = String(option.id)
    specNames.push(option.tag_name)
    extraPrice += option.extra_price || 0
  })

  const summary = specNames.join(' / ')
  if (summary) {
    customizations.meta_specs = summary
  }

  return {
    customizations,
    summary,
    extraPrice
  }
}

function syncDishCustomizationSelection(dish: DishOption): DishOption {
  if (!dish.checked || !Array.isArray(dish.customization_groups) || dish.customization_groups.length === 0) {
    return {
      ...dish,
      customization_error_message: ''
    }
  }

  const { selectedSpecs, hasInvalidSelection } = buildSelectedSpecState(dish.customization_groups, dish.customizations || {})
  const payload = buildDishCustomizationPayload(dish.customization_groups, selectedSpecs)

  return {
    ...dish,
    customizations: payload.customizations,
    customization_summary: payload.summary,
    customization_extra_price: payload.extraPrice,
    customization_error_message: hasInvalidSelection ? '已保存规格已变化，已按当前规格重置。' : ''
  }
}

function buildSelectedTagState(selectedTagIds: number[]): Record<string, boolean> {
  return selectedTagIds.reduce<Record<string, boolean>>((result, id) => {
    result[String(id)] = true
    return result
  }, {})
}

function formatFenToYuanInput(value?: number): string {
  if (!Number.isFinite(value) || !value) {
    return ''
  }

  return (Number(value) / 100).toFixed(2)
}

function parsePriceInputToFen(value: string): number {
  const parsed = Number.parseFloat((value || '').trim())
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }

  return Math.round(parsed * 100)
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    loadWarningMessage: '',
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
    selectedTagState: {} as Record<string, boolean>,
    tagSubmitting: false,
    createPopupVisible: false,
    createPopupMode: '' as CreatePopupMode,
    createInputValue: '',
    selectedDishIds: [] as number[],
    allDishes: [] as DishOption[],
    dishes: [] as DishOption[],
    customPriceValue: '',
    comboPriceCustomized: false,
    isComboOnline: true,
    autoName: '精选套餐',
    originalTotal: 0,
    comboPricePreview: 0,
    selectedDishQuantityTotal: 0,
    selectedDishPreviews: [] as string[],
    dishEmptyDescription: '暂无上架且可售的菜品，请先去上架可售菜品'
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
        customPriceValue: formatFenToYuanInput(combo.combo_price),
        comboPriceCustomized: true,
        isComboOnline: !!combo.is_online,
        allDishes: allDishes.map((dish) => syncDishCustomizationSelection(dish))
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
      comboId
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
      initialErrorMessage: '',
      loadWarningMessage: ''
    })
    try {
      const [allDishesResult, comboResult, tagsResult] = await settleAll([
        this.fetchAllDishes(),
        this.data.isEdit
          ? ComboManagementService.getComboDetail(this.data.comboId)
          : Promise.resolve(null as ComboSetWithDetailsResponse | null),
        TagService.listTags('combo')
      ] as const)

      if (allDishesResult.status !== 'fulfilled') {
        throw allDishesResult.reason
      }
      if (this.data.isEdit && comboResult.status !== 'fulfilled') {
        throw comboResult.reason
      }

      const allDishesResponse = allDishesResult.value
      const comboRes = comboResult.status === 'fulfilled' ? comboResult.value : null
      const availableTags = tagsResult.status === 'fulfilled' ? tagsResult.value : []
      const loadWarningMessage = tagsResult.status === 'rejected'
        ? '套餐标签未同步，仍可继续编辑套餐内容'
        : ''

      const dishes = allDishesResponse.map((dish: DishResponse) => ({
        id: dish.id,
        name: dish.name,
        price: dish.price,
        is_available: dish.is_available,
        is_online: dish.is_online,
        image_url: getPublicImageUrl(dish.image_url || ''),
        checked: false,
        quantity: 1,
        customization_groups: Array.isArray(dish.customization_groups) ? dish.customization_groups : [],
        customization_error_message: '',
        customizations: {},
        customization_summary: '',
        customization_extra_price: 0
      }))

      const selectedDishMap = new Map<number, SelectedComboDishState>(
        (comboRes?.dishes || []).map((dish: ComboSetWithDetailsResponse['dishes'][number]) => [dish.dish_id, {
          quantity: normalizeDishQuantity(dish.quantity),
          customizations: normalizeComboCustomizations(dish.customizations),
          customizationSummary: dish.customization_summary || '',
          customizationExtraPrice: dish.customization_extra_price || 0
        }])
      )
      const selectedDishIds = Array.from(selectedDishMap.keys()) as number[]

      const selectedSet = new Set(selectedDishIds)
      const dishOptions = dishes.map((dish) => {
        const selectedDishState = selectedDishMap.get(dish.id)
        return syncDishCustomizationSelection({
          ...dish,
          checked: selectedSet.has(dish.id),
          quantity: selectedDishState?.quantity || 1,
          customizations: selectedDishState?.customizations || {},
          customization_summary: selectedDishState?.customizationSummary || '',
          customization_extra_price: selectedDishState?.customizationExtraPrice || 0
        })
      })

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
            .map((tag: TagInfo) => Number(tag.id))
            .filter((id: number) => Number.isFinite(id) && id > 0)
          : [],
        selectedTagState: buildSelectedTagState(
          Array.isArray(comboRes?.tags)
            ? comboRes.tags
              .map((tag: TagInfo) => Number(tag.id))
              .filter((id: number) => Number.isFinite(id) && id > 0)
            : []
        ),
        customPriceValue: formatFenToYuanInput(comboRes?.combo_price || 0),
        comboPriceCustomized: !!comboRes?.combo_price,
        isComboOnline: comboRes?.is_online !== false,
        loadWarningMessage,
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
  const allDishes = this.data.allDishes.map((dish) => syncDishCustomizationSelection({
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

  applyDishCustomizationPayload(index: number, selectedSpecs: SelectedSpecState, errorMessage = '') {
    const dish = this.data.allDishes[index]
    if (!dish) {
      return
    }

    const payload = buildDishCustomizationPayload(dish.customization_groups || [], selectedSpecs)
    this.setData({
      [`allDishes[${index}].customizations`]: payload.customizations,
      [`allDishes[${index}].customization_summary`]: payload.summary,
      [`allDishes[${index}].customization_extra_price`]: payload.extraPrice,
      [`allDishes[${index}].customization_error_message`]: errorMessage
    }, () => {
      this.syncVisibleDishes()
      this.recomputePreview()
    })
  },

  onComboPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    this.setData({
      customPriceValue: (e.detail.value || '').trim(),
      comboPriceCustomized: true
    }, () => {
      this.recomputePreview()
    })
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

  onTagToggle(e: WechatMiniprogram.CustomEvent) {
    const { tagId } = e.currentTarget.dataset as { tagId?: number }
    if (typeof tagId !== 'number') {
      return
    }

    const detail = e.detail as boolean | { checked?: boolean } | undefined
    const nextChecked = typeof detail === 'boolean'
      ? detail
      : !!detail?.checked

    const selectedTagIds = nextChecked
      ? (this.data.selectedTagIds.includes(tagId)
        ? this.data.selectedTagIds
        : [...this.data.selectedTagIds, tagId])
      : this.data.selectedTagIds.filter((id) => id !== tagId)

    this.setData({
      selectedTagIds,
      selectedTagState: buildSelectedTagState(selectedTagIds)
    })
  },

  onCreateTag() {
    if (this.data.tagSubmitting) {
      return
    }

    this.setData({
      createPopupVisible: true,
      createPopupMode: 'tag',
      createInputValue: ''
    })
  },

  onCloseCreatePopup() {
    if (this.data.tagSubmitting) {
      return
    }

    this.setData({
      createPopupVisible: false,
      createPopupMode: '',
      createInputValue: ''
    })
  },

  onCreateInputChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    this.setData({ createInputValue: (e.detail.value || '').replace(/^\s+/, '') })
  },

  async onConfirmCreatePopup() {
    if (this.data.tagSubmitting || this.data.createPopupMode !== 'tag') {
      return
    }

    const name = this.data.createInputValue.trim()
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
        createPopupVisible: false,
        createPopupMode: '',
        createInputValue: '',
        availableTags,
        selectedTagIds,
        selectedTagState: buildSelectedTagState(selectedTagIds),
        loadWarningMessage: ''
      })
    } catch (err) {
      logger.error('Create combo tag failed', err)
      wx.showToast({ title: getErrorMessage(err, '新增标签失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ tagSubmitting: false })
    }
  },

  onComboOnlineSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    this.setData({ isComboOnline: !!e.detail?.value })
  },

  onDishSpecTagToggle(
    e: WechatMiniprogram.CustomEvent & {
      currentTarget: { dataset: { id?: number, groupId?: string | number, specId?: string | number, required?: string | number | boolean } }
    }
  ) {
    const dishId = Number(e.currentTarget.dataset?.id || 0)
    const groupId = String(e.currentTarget.dataset?.groupId || '')
    const specId = String(e.currentTarget.dataset?.specId || '')
    const requiredFlag = e.currentTarget.dataset?.required
    const isRequired = requiredFlag === true || requiredFlag === '1' || requiredFlag === 1
    if (!dishId || !groupId || !specId) {
      return
    }

    const index = this.data.allDishes.findIndex((dish) => dish.id === dishId)
    if (index < 0) {
      return
    }

    const dish = this.data.allDishes[index]

    const detail = e.detail as boolean | { checked?: boolean } | undefined
    const nextChecked = typeof detail === 'boolean'
      ? detail
      : !!detail?.checked

    const { selectedSpecs } = buildSelectedSpecState(dish.customization_groups || [], dish.customizations || {})
    if (nextChecked) {
      selectedSpecs[groupId] = specId
    } else if (!isRequired) {
      delete selectedSpecs[groupId]
    }

    this.applyDishCustomizationPayload(index, selectedSpecs)
  },

  calcOriginalTotal() {
    return this.data.allDishes.reduce((sum, dish) => {
      if (!dish.checked) {
        return sum
      }
      return sum + (dish.price + (dish.customization_extra_price || 0)) * normalizeDishQuantity(dish.quantity)
    }, 0)
  },

  calcComboPrice(originalTotal: number) {
    const parsedComboPrice = parsePriceInputToFen(this.data.customPriceValue)
    if (parsedComboPrice > 0) {
      return parsedComboPrice
    }

    if (!this.data.comboPriceCustomized) {
      return originalTotal
    }

    return 0
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
    if (comboPrice <= 0) {
      wx.showToast({ title: '套餐价必须大于0', icon: 'none' })
      return
    }

    const autoName = this.buildAutoName()
    const name = this.buildComboName(autoName)
    const description = this.data.comboDescription.trim()
    const isOnline = this.data.isComboOnline
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

    if (!this.data.comboPriceCustomized) {
      updates.customPriceValue = formatFenToYuanInput(originalTotal)
    }

    if (!this.data.comboNameCustomized) {
      updates.comboName = autoName
    }

    this.setData(updates)
  },

  syncVisibleDishes() {
    const dishes = this.data.allDishes.filter((dish) => (dish.is_online && dish.is_available) || dish.checked)
    const activeDishCount = this.data.allDishes.filter((dish) => dish.is_online && dish.is_available).length
    const dishEmptyDescription = this.data.allDishes.length === 0
      ? '暂无可选菜品，请先创建菜品'
      : activeDishCount === 0
        ? '暂无上架且可售的菜品，请先去上架可售菜品'
        : '暂无可选菜品，请先创建菜品'
    this.setData({ dishes, dishEmptyDescription })
  }
})
