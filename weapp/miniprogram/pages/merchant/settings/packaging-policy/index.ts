import {
  getMyMerchantPackagingPolicy,
  MerchantPackagingPolicyResponse,
  PackagingPolicyCandidateDish,
  PackagingPolicyOrderType,
  updateMyMerchantPackagingPolicy
} from '../../../../api/merchant'
import { DishManagementService, DishResponse } from '../../../../api/dish'
import { logger } from '../../../../utils/logger'
import { settleAll } from '../../../../utils/promise'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

type PackagingOrderTypeState = Record<PackagingPolicyOrderType, boolean>

interface PackagingPolicyFormState {
  orderTypes: PackagingOrderTypeState
  selectedDishIds: number[]
}

interface PackagingDishOption {
  id: number
  name: string
  price: number
  is_online: boolean
  is_available: boolean
  selected: boolean
}

type PackagingDishSource = Pick<DishResponse, 'id' | 'name' | 'price' | 'is_online' | 'is_available'>

const PACKAGING_POLICY_AUTO_REFRESH_WINDOW_MS = 60 * 1000

const ORDER_TYPE_OPTIONS: Array<{ key: PackagingPolicyOrderType, label: string, desc: string }> = [
  { key: 'takeout', label: '外卖订单', desc: '顾客选择配送到家时，订单必须命中 1 个包装菜品。' },
  { key: 'takeaway', label: '到店自取', desc: '顾客选择打包自取时，订单必须命中 1 个包装菜品。' }
]

function createOrderTypeState(selected: PackagingPolicyOrderType[]) {
  const selectedSet = new Set(selected)
  return {
    takeout: selectedSet.has('takeout'),
    takeaway: selectedSet.has('takeaway')
  }
}

function normalizeSelectedDishIds(ids: number[]) {
  return Array.from(new Set(ids)).sort((left, right) => left - right)
}

function buildForm(policy: MerchantPackagingPolicyResponse): PackagingPolicyFormState {
  return {
    orderTypes: createOrderTypeState(policy.applicable_order_types || []),
    selectedDishIds: normalizeSelectedDishIds(policy.candidate_dish_ids || [])
  }
}

function extractDishOptions(dishes: PackagingDishSource[]): PackagingDishOption[] {
  return dishes.map((dish) => ({
    id: dish.id,
    name: dish.name,
    price: dish.price,
    is_online: dish.is_online,
    is_available: dish.is_available,
    selected: false
  }))
}

function mergePackagingDishOptions(
  selectedDishIds: number[],
  fetchedDishes: PackagingDishSource[],
  candidateDishes: PackagingPolicyCandidateDish[]
) {
  const selectedSet = new Set(selectedDishIds)
  const optionMap = new Map<number, PackagingDishOption>()

  for (const item of extractDishOptions(fetchedDishes)) {
    optionMap.set(item.id, {
      ...item,
      selected: selectedSet.has(item.id)
    })
  }

  for (const item of candidateDishes || []) {
    if (!optionMap.has(item.id)) {
      optionMap.set(item.id, {
        id: item.id,
        name: item.name,
        price: item.price,
        is_online: item.is_online,
        is_available: item.is_available,
        selected: selectedSet.has(item.id)
      })
    }
  }

  return Array.from(optionMap.values()).sort((left, right) => {
    if (left.selected !== right.selected) {
      return left.selected ? -1 : 1
    }
    if (left.is_online !== right.is_online) {
      return left.is_online ? -1 : 1
    }
    return left.name.localeCompare(right.name, 'zh-CN')
  })
}

function hasFormChanged(current: PackagingPolicyFormState, initial: PackagingPolicyFormState) {
  return JSON.stringify(current) !== JSON.stringify(initial)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function toDishSources(dishes: Array<PackagingDishOption | PackagingPolicyCandidateDish>): PackagingDishSource[] {
  return dishes.map((dish) => ({
    id: dish.id,
    name: dish.name,
    price: dish.price,
    is_online: dish.is_online,
    is_available: dish.is_available
  }))
}

async function loadAllMerchantDishes() {
  const dishes: DishResponse[] = []
  let pageId = 1
  const pageSize = 100
  let hasMore = true

  while (hasMore) {
    const response = await DishManagementService.listDishes({
      page_id: pageId,
      page_size: pageSize
    })
    const pageDishes = Array.isArray(response.dishes)
      ? response.dishes.filter((dish): dish is DishResponse => !!dish)
      : []

    dishes.push(...pageDishes)
    hasMore = !(pageDishes.length < pageSize || dishes.length >= (response.total || 0))
    if (hasMore) {
      pageId += 1
    }
  }

  return dishes
}

function getSelectedOrderTypes(orderTypes: PackagingOrderTypeState): PackagingPolicyOrderType[] {
  return ORDER_TYPE_OPTIONS.filter((item) => orderTypes[item.key]).map((item) => item.key)
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    orderTypeOptions: ORDER_TYPE_OPTIONS,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    actionNoticeMessage: '',
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    merchantId: 0,
    lastPolicyLoadedAt: 0,
    dishesLoaded: false,
    dishesLoading: false,
    dishesError: false,
    dishesErrorMessage: '',
    form: buildForm({
      merchant_id: 0,
      applicable_order_types: [],
      candidate_dish_ids: [],
      candidate_dishes: []
    }) as PackagingPolicyFormState,
    initialForm: buildForm({
      merchant_id: 0,
      applicable_order_types: [],
      candidate_dish_ids: [],
      candidate_dishes: []
    }) as PackagingPolicyFormState,
    dishOptions: [] as PackagingDishOption[],
    hasChanges: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    this.loadPolicy({ force: true, refreshDishes: true })
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      if (shouldAutoRefresh(this.data.lastPolicyLoadedAt, PACKAGING_POLICY_AUTO_REFRESH_WINDOW_MS)) {
        this.loadPolicy({ showLoading: false })
        return
      }

      if (!this.data.dishesLoaded) {
        this.loadDishOptions().catch((err) => logger.error('Load merchant packaging dishes onShow failed', err))
      }
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }
    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      wx.showToast({ title: '当前有未保存修改，请先保存后再刷新', icon: 'none' })
      return
    }
    this.loadPolicy({ showLoading: false, force: true, refreshDishes: true })
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadPolicy({ showLoading: false, force: true, refreshDishes: true })
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })
    this.onLoad()
  },

  async loadPolicy(options: { showLoading?: boolean, force?: boolean, refreshDishes?: boolean } = {}) {
    if (this.data.loading) return

    const showLoading = options.showLoading ?? true
    const force = options.force === true
    const refreshDishes = options.refreshDishes === true
    const hasExistingData = !this.data.initialLoading
    const isSilentRefresh = !showLoading && hasExistingData

    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastPolicyLoadedAt, PACKAGING_POLICY_AUTO_REFRESH_WINDOW_MS)) {
      if (!this.data.dishesLoaded) {
        this.loadDishOptions().catch((err) => logger.error('Load merchant packaging dishes after skipped refresh failed', err))
      }
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    let shouldLoadDishes = false

    try {
      const policy = await getMyMerchantPackagingPolicy()
      const form = buildForm(policy)
      const nextState: Record<string, unknown> = {
        merchantId: policy.merchant_id,
        form,
        initialForm: JSON.parse(JSON.stringify(form)),
        actionNoticeMessage: '',
        hasChanges: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastPolicyLoadedAt: Date.now(),
        dishOptions: mergePackagingDishOptions(
          form.selectedDishIds,
          toDishSources(this.data.dishOptions),
          policy.candidate_dishes || []
        )
      }

      this.setData(nextState)
      shouldLoadDishes = refreshDishes || !this.data.dishesLoaded
    } catch (err: unknown) {
      logger.error('Load merchant packaging policy failed', err)
      const message = getErrorMessage(err, '包装费策略加载失败，请稍后重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (hasExistingData) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }

    if (shouldLoadDishes) {
      this.loadDishOptions({ force: refreshDishes }).catch((err) => logger.error('Load merchant packaging dishes failed', err))
    }
  },

  async loadDishOptions(options: { force?: boolean } = {}) {
    if (this.data.dishesLoading) return

    const force = options.force === true
    if (!force && this.data.dishesLoaded) return

    this.setData({
      dishesLoading: true,
      dishesError: false,
      dishesErrorMessage: ''
    })

    try {
      const dishes = await loadAllMerchantDishes()
      this.setData({
        dishOptions: mergePackagingDishOptions(
          this.data.form.selectedDishIds,
          dishes,
          toDishSources(this.data.dishOptions)
        ),
        dishesLoaded: true,
        dishesError: false,
        dishesErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load merchant packaging dishes failed', err)
      this.setData({
        dishesLoaded: false,
        dishesError: true,
        dishesErrorMessage: getErrorMessage(err, '包装菜品列表加载失败，请稍后重试')
      })
    } finally {
      this.setData({ dishesLoading: false })
    }
  },

  onToggleOrderType(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key: PackagingPolicyOrderType }
    const form = {
      ...this.data.form,
      orderTypes: {
        ...this.data.form.orderTypes,
        [key]: !this.data.form.orderTypes[key]
      }
    }
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      form,
      hasChanges: hasFormChanged(form, this.data.initialForm)
    })
  },

  onToggleDish(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const selectedSet = new Set(this.data.form.selectedDishIds)
    if (selectedSet.has(id)) {
      selectedSet.delete(id)
    } else {
      selectedSet.add(id)
    }

    const selectedDishIds = normalizeSelectedDishIds(Array.from(selectedSet))
    const form = {
      ...this.data.form,
      selectedDishIds
    }

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      form,
      dishOptions: this.data.dishOptions.map((item) => item.id === id ? { ...item, selected: !item.selected } : item),
      hasChanges: hasFormChanged(form, this.data.initialForm)
    })
  },

  validateForm() {
    const selectedOrderTypes = getSelectedOrderTypes(this.data.form.orderTypes)
    const selectedDishCount = this.data.form.selectedDishIds.length

    if (selectedOrderTypes.length > 0 && selectedDishCount === 0) {
      wx.showToast({ title: '请至少选择 1 个包装菜品', icon: 'none' })
      return false
    }

    if (selectedOrderTypes.length === 0 && selectedDishCount > 0) {
      wx.showToast({ title: '请先选择适用订单类型', icon: 'none' })
      return false
    }

    return true
  },

  async onSave() {
    if (this.data.saving || !this.data.hasChanges) return
    if (!this.validateForm()) return

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const updated = await updateMyMerchantPackagingPolicy({
        applicable_order_types: getSelectedOrderTypes(this.data.form.orderTypes),
        candidate_dish_ids: this.data.form.selectedDishIds
      })

      const form = buildForm(updated)
      this.setData({
        merchantId: updated.merchant_id,
        form,
        initialForm: JSON.parse(JSON.stringify(form)),
        actionNoticeMessage: '包装费策略已保存。',
        dishOptions: mergePackagingDishOptions(
          form.selectedDishIds,
          this.data.dishOptions.map((item) => ({
            id: item.id,
            name: item.name,
            price: item.price,
            is_online: item.is_online,
            is_available: item.is_available
          })),
          updated.candidate_dishes || []
        ),
        hasChanges: false,
        refreshErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Save merchant packaging policy failed', err)
      wx.showToast({ title: getErrorMessage(err, '保存失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  onRetryDishes() {
    this.loadDishOptions({ force: true })
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadPolicy({ force: true, refreshDishes: true })
  }
})