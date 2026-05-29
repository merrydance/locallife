import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import { getStableBarHeights } from '../../../utils/responsive'
import { DailyInventoryWithDishResponse, InventoryManagementService } from '../_main_shared/api/dish'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'

interface InventoryViewItem extends DailyInventoryWithDishResponse {
  draft_total_quantity: number
  draft_total_quantity_text: string
  draft_available: number
  save_disabled: boolean
  submitting: boolean
}

interface LoadInventoryOptions {
  preserveCurrent?: boolean
  usePageLoading?: boolean
  keepVisibleData?: boolean
}

function toSafeNumber(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function calculateAvailable(totalQuantity: number, soldQuantity: number, reservedQuantity: number): number {
  if (totalQuantity === -1) return -1
  const available = totalQuantity - soldQuantity - reservedQuantity
  return available < 0 ? 0 : available
}

function formatDraftQuantityText(totalQuantity: number): string {
  if (totalQuantity === -1) {
    return ''
  }

  return String(totalQuantity)
}

function normalizeDraftQuantity(rawValue: string): string {
  return String(rawValue || '').replace(/\s+/g, '')
}

function buildInventoryLimitText(totalQuantity: number): string {
  if (totalQuantity === -1) {
    return '今日限售 不限'
  }

  return `今日限售 ${totalQuantity} 份`
}

function shouldDisableSave(item: Pick<InventoryViewItem, 'submitting' | 'draft_total_quantity' | 'total_quantity' | 'draft_total_quantity_text'>): boolean {
  if (item.submitting) {
    return true
  }

  if (item.draft_total_quantity === -1) {
    return item.total_quantity === -1
  }

  const normalizedValue = normalizeDraftQuantity(item.draft_total_quantity_text)
  if (!normalizedValue) {
    return true
  }

  const parsed = Number(normalizedValue)
  if (!Number.isInteger(parsed) || parsed < 0) {
    return true
  }

  return parsed === item.total_quantity
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    switchingDate: false,
    lastLoadedAt: 0,
    date: '',
    datePickerVisible: false,
    hasInventories: false,
    inventories: [] as InventoryViewItem[]
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    const today = dayjs().format('YYYY-MM-DD')
    this.setData({ navBarHeight, date: today })

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

    void this.loadInventory(today)
  },

  onShow() {
    if (
      !this.data.accessReady ||
      this.data.accessDenied ||
      this.data.accessErrorMessage ||
      this.data.initialLoading ||
      this.data.loading ||
      !Array.isArray(this.data.inventories)
    ) {
      if (!Array.isArray(this.data.inventories)) {
        this.setData({
          inventories: [],
          hasInventories: false
        })
      }
      return
    }

    const today = dayjs().format('YYYY-MM-DD')
    if (this.data.date !== today) {
      return
    }

    if (Date.now() - this.data.lastLoadedAt < 60 * 1000) {
      return
    }

    void this.loadInventory(this.data.date, { preserveCurrent: true, usePageLoading: false })
  },

  async loadInventory(date: string, options?: LoadInventoryOptions) {
    if (this.data.loading || !this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    const preserveCurrent = !!options?.preserveCurrent && this.data.date === date && this.data.inventories.length > 0
    const keepVisibleData = !!options?.keepVisibleData && this.data.inventories.length > 0
    const usePageLoading = options?.usePageLoading !== false && !preserveCurrent && !keepVisibleData

    this.setData({
      loading: true,
      ...(usePageLoading
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          }
        : preserveCurrent || keepVisibleData
          ? {
              refreshErrorMessage: ''
            }
        : {
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          })
    })
    try {
      const response = await InventoryManagementService.getDailyInventory(date)

      const rawList = Array.isArray(response?.inventories)
        ? (response.inventories as Array<DailyInventoryWithDishResponse | null>)
        : []

      const inventories = rawList
        .filter((item): item is DailyInventoryWithDishResponse => !!item && typeof item === 'object')
        .map((item) => {
          const totalQuantity = toSafeNumber(item.total_quantity, -1)
          const soldQuantity = toSafeNumber(item.sold_quantity, 0)
          const reservedQuantity = toSafeNumber(item.reserved_quantity, 0)

          return {
            ...item,
            total_quantity: totalQuantity,
            sold_quantity: soldQuantity,
            reserved_quantity: reservedQuantity,
            draft_total_quantity: totalQuantity,
            draft_total_quantity_text: formatDraftQuantityText(totalQuantity),
            draft_available: calculateAvailable(totalQuantity, soldQuantity, reservedQuantity),
            save_disabled: true,
            submitting: false
          }
        })

      this.setData({
        date,
        inventories,
        hasInventories: inventories.length > 0,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load inventory failed', err)
      const message = getErrorUserMessage(err, '加载库存失败，请稍后重试')

      if (!preserveCurrent) {
        if (keepVisibleData) {
          this.setData({
            initialLoading: false,
            refreshErrorMessage: `${message}，当前已保留${this.data.date}的库存结果`
          })
        } else {
          this.setData({
            inventories: [],
            hasInventories: false,
            initialLoading: false,
            initialError: true,
            initialErrorMessage: message
          })
        }
      } else {
        this.setData({
          initialLoading: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      this.setData({ loading: false, initialLoading: false, switchingDate: false })
      wx.stopPullDownRefresh()
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadInventory(this.data.date, { preserveCurrent: true, usePageLoading: false })
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
    void this.onLoad()
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    void this.loadInventory(this.data.date)
  },

  onRetryRefresh() {
    void this.loadInventory(this.data.date, { preserveCurrent: true, usePageLoading: false })
  },

  onOpenDatePicker() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.loading) {
      return
    }

    this.setData({ datePickerVisible: true })
  },

  onCloseDatePicker() {
    this.setData({ datePickerVisible: false })
  },

  applyDateSelection(date: string) {
    if (!date || date === this.data.date || !this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }

    this.setData({
      datePickerVisible: false,
      switchingDate: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })
    void this.loadInventory(date, { usePageLoading: false, keepVisibleData: true })
  },

  onDateConfirm(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const date = e.detail?.value || ''
    this.applyDateSelection(date)
  },

  commitQuantityDraft(index: number, rawValue?: string) {
    const current = this.data.inventories[index]
    if (!current) {
      return false
    }

    const value = normalizeDraftQuantity(rawValue ?? current.draft_total_quantity_text)
    if (!value) {
      this.setData({
        [`inventories[${index}].draft_total_quantity_text`]: '',
        [`inventories[${index}].save_disabled`]: true
      })
      return false
    }

    const parsed = Number(value)
    if (!Number.isInteger(parsed) || parsed < 0) {
      wx.showToast({ title: '请输入0或正整数', icon: 'none' })
      const fallbackText = formatDraftQuantityText(current.draft_total_quantity)
      this.setData({
        [`inventories[${index}].draft_total_quantity_text`]: fallbackText,
        [`inventories[${index}].save_disabled`]: shouldDisableSave({
          ...current,
          draft_total_quantity_text: fallbackText,
          submitting: current.submitting
        })
      })
      return false
    }

    const draftAvailable = calculateAvailable(parsed, current.sold_quantity, current.reserved_quantity || 0)
    const nextDraftText = String(parsed)
    const nextDraftQuantity = parsed
    this.setData({
      [`inventories[${index}].draft_total_quantity`]: nextDraftQuantity,
      [`inventories[${index}].draft_total_quantity_text`]: nextDraftText,
      [`inventories[${index}].draft_available`]: draftAvailable,
      [`inventories[${index}].save_disabled`]: shouldDisableSave({
        ...current,
        draft_total_quantity: nextDraftQuantity,
        draft_total_quantity_text: nextDraftText,
        submitting: current.submitting
      })
    })
    return true
  },

  onQuantityDraftChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { index } = e.currentTarget.dataset as { index?: number }
    if (index === undefined) return

    const value = normalizeDraftQuantity(e.detail?.value || '')
    const current = this.data.inventories[index]
    if (!current) {
      return
    }

    this.setData({
      [`inventories[${index}].draft_total_quantity_text`]: value,
      [`inventories[${index}].save_disabled`]: shouldDisableSave({
        ...current,
        draft_total_quantity_text: value,
        submitting: current.submitting
      })
    })
  },

  onQuantityInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { index } = e.currentTarget.dataset as { index?: number }
    if (index === undefined) return

    this.commitQuantityDraft(index, e.detail?.value)
  },

  onUnlimitedChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { index } = e.currentTarget.dataset as { index?: number }
    if (index === undefined) return

    const checked = !!e.detail?.value
    const current = this.data.inventories[index]
    if (checked) {
      this.setData({
        [`inventories[${index}].draft_total_quantity`]: -1,
        [`inventories[${index}].draft_total_quantity_text`]: '',
        [`inventories[${index}].draft_available`]: -1,
        [`inventories[${index}].save_disabled`]: shouldDisableSave({
          ...current,
          draft_total_quantity: -1,
          draft_total_quantity_text: '',
          submitting: current.submitting
        })
      })
      return
    }

    const fallback = current.total_quantity >= 0 ? current.total_quantity : 0
    this.setData({
      [`inventories[${index}].draft_total_quantity`]: fallback,
      [`inventories[${index}].draft_total_quantity_text`]: String(fallback),
      [`inventories[${index}].draft_available`]: calculateAvailable(fallback, current.sold_quantity, current.reserved_quantity || 0),
      [`inventories[${index}].save_disabled`]: shouldDisableSave({
        ...current,
        draft_total_quantity: fallback,
        draft_total_quantity_text: String(fallback),
        submitting: current.submitting
      })
    })
  },

  async onSave(e: WechatMiniprogram.TouchEvent) {
    const { index } = e.currentTarget.dataset as { index?: number }
    if (index === undefined) return

    this.commitQuantityDraft(index, this.data.inventories[index]?.draft_total_quantity_text)

    const item = this.data.inventories[index]
    if (item.draft_total_quantity !== -1 && !item.draft_total_quantity_text) {
      wx.showToast({ title: '请输入库存数量', icon: 'none' })
      return
    }

    if (item.draft_total_quantity === item.total_quantity) {
      return
    }

    this.setData({
      [`inventories[${index}].submitting`]: true,
      [`inventories[${index}].save_disabled`]: true
    })
    try {
      const updated = await InventoryManagementService.updateInventory({
        date: this.data.date,
        dish_id: item.dish_id,
        total_quantity: item.draft_total_quantity
      })

      const soldQuantity = toSafeNumber(updated.sold_quantity, item.sold_quantity)
      const reservedQuantity = toSafeNumber(updated.reserved_quantity, item.reserved_quantity)
      const totalQuantity = toSafeNumber(updated.total_quantity, item.total_quantity)
      const available = typeof updated.available === 'number' && Number.isFinite(updated.available)
        ? updated.available
        : calculateAvailable(totalQuantity, soldQuantity, reservedQuantity)

      this.setData({
        [`inventories[${index}].sold_quantity`]: soldQuantity,
        [`inventories[${index}].reserved_quantity`]: reservedQuantity,
        [`inventories[${index}].total_quantity`]: totalQuantity,
        [`inventories[${index}].available`]: available,
        [`inventories[${index}].draft_total_quantity`]: totalQuantity,
        [`inventories[${index}].draft_total_quantity_text`]: formatDraftQuantityText(totalQuantity),
        [`inventories[${index}].draft_available`]: available,
        [`inventories[${index}].save_disabled`]: true,
        [`inventories[${index}].submitting`]: false,
        lastLoadedAt: Date.now()
      })
      wx.showToast({
        title: `${item.dish_name}已保存为${buildInventoryLimitText(totalQuantity)}`,
        icon: 'success',
        duration: 1800
      })
    } catch (err) {
      logger.error('Update inventory failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '更新失败，请稍后重试'), icon: 'none' })
    } finally {
      const latest = this.data.inventories[index]
      if (latest) {
        this.setData({
          [`inventories[${index}].submitting`]: false,
          [`inventories[${index}].save_disabled`]: shouldDisableSave({
            ...latest,
            submitting: false
          })
        })
      }
    }
  }
})
