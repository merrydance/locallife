import dayjs from 'dayjs'
import { getStableBarHeights } from '../../../utils/responsive'
import { DailyInventoryWithDishResponse, InventoryManagementService } from '../../../api/dish'
import { logger } from '../../../utils/logger'

interface InventoryViewItem extends DailyInventoryWithDishResponse {
  draft_total_quantity: number
  draft_available: number
  submitting: boolean
}

function toSafeNumber(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function calculateAvailable(totalQuantity: number, soldQuantity: number, reservedQuantity: number): number {
  if (totalQuantity === -1) return -1
  const available = totalQuantity - soldQuantity - reservedQuantity
  return available < 0 ? 0 : available
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    date: '',
    hasInventories: false,
    inventories: [] as InventoryViewItem[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    const today = dayjs().format('YYYY-MM-DD')
    this.setData({ navBarHeight, date: today })
    this.loadInventory(today)
  },

  onShow() {
    if (!Array.isArray(this.data.inventories)) {
      this.setData({
        inventories: [],
        hasInventories: false
      })
    }
  },

  async loadInventory(date: string) {
    if (this.data.loading) return

    this.setData({ loading: true })
    try {
      const response = await InventoryManagementService.getDailyInventory(date)

      const rawList = Array.isArray((response as any)?.inventories)
        ? ((response as any).inventories as Array<DailyInventoryWithDishResponse | null>)
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
            draft_available: calculateAvailable(totalQuantity, soldQuantity, reservedQuantity),
            submitting: false
          }
        })

      this.setData({
        inventories,
        hasInventories: inventories.length > 0
      })
    } catch (err) {
      logger.error('Load inventory failed', err)
      this.setData({
        inventories: [],
        hasInventories: false
      })
      wx.showToast({ title: '加载库存失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onPullDownRefresh() {
    this.loadInventory(this.data.date)
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const date = e.detail.value
    this.setData({ date })
    this.loadInventory(date)
  },

  onQuantityInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { index } = e.currentTarget.dataset as { index?: number }
    if (index === undefined) return

    const value = (e.detail?.value || '').trim()
    if (!value) return

    const parsed = Number(value)
    if (!Number.isInteger(parsed) || parsed < 0) {
      wx.showToast({ title: '请输入0或正整数', icon: 'none' })
      return
    }

    const current = this.data.inventories[index]
    const draftAvailable = calculateAvailable(parsed, current.sold_quantity, current.reserved_quantity || 0)
    this.setData({
      [`inventories[${index}].draft_total_quantity`]: parsed,
      [`inventories[${index}].draft_available`]: draftAvailable
    })
  },

  onUnlimitedChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { index } = e.currentTarget.dataset as { index?: number }
    if (index === undefined) return

    const checked = !!e.detail?.value
    const current = this.data.inventories[index]
    if (checked) {
      this.setData({
        [`inventories[${index}].draft_total_quantity`]: -1,
        [`inventories[${index}].draft_available`]: -1
      })
      return
    }

    const fallback = current.total_quantity >= 0 ? current.total_quantity : 0
    this.setData({
      [`inventories[${index}].draft_total_quantity`]: fallback,
      [`inventories[${index}].draft_available`]: calculateAvailable(fallback, current.sold_quantity, current.reserved_quantity || 0)
    })
  },

  async onSave(e: WechatMiniprogram.TouchEvent) {
    const { index } = e.currentTarget.dataset as { index?: number }
    if (index === undefined) return

    const item = this.data.inventories[index]
    if (item.draft_total_quantity === item.total_quantity) {
      wx.showToast({ title: '库存未变更', icon: 'none' })
      return
    }

    this.setData({ [`inventories[${index}].submitting`]: true })
    try {
      const updated = await InventoryManagementService.updateInventory({
        date: this.data.date,
        dish_id: item.dish_id,
        total_quantity: item.draft_total_quantity
      })
      this.setData({
        [`inventories[${index}].total_quantity`]: updated.total_quantity,
        [`inventories[${index}].available`]: updated.available,
        [`inventories[${index}].draft_total_quantity`]: updated.total_quantity,
        [`inventories[${index}].draft_available`]: updated.available
      })
      wx.showToast({ title: '库存已更新', icon: 'success' })
    } catch (err) {
      logger.error('Update inventory failed', err)
      wx.showToast({ title: '更新失败', icon: 'none' })
    } finally {
      this.setData({ [`inventories[${index}].submitting`]: false })
    }
  }
})
