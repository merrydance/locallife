import { getStableBarHeights } from '../../../utils/responsive'
import { ComboManagementService, ComboSetResponse } from '../../../api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { logger } from '../../../utils/logger'

interface ComboViewItem extends ComboSetResponse {
  coverImageUrl: string
  imageCount: number
  submitting: boolean
}

function normalizeComboImages(urls?: string[]): string[] {
  if (!Array.isArray(urls)) return []
  return urls.map((url) => getPublicImageUrl(url)).filter((url) => !!url)
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    combos: [] as ComboViewItem[],
    pageId: 1,
    pageSize: 20,
    hasMore: true
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadCombos(true)
  },

  onShow() {
    this.loadCombos(true)
  },

  async loadCombos(reset = false) {
    if (this.data.loading) return
    if (!reset && !this.data.hasMore) return

    this.setData({ loading: true })
    try {
      const pageId = reset ? 1 : this.data.pageId
      const response = await ComboManagementService.listCombos({
        page_id: pageId,
        page_size: this.data.pageSize
      })

      const combos = (response.combo_sets || []).map((combo) => ({
        ...combo,
        dish_image_urls: normalizeComboImages(combo.dish_image_urls),
        coverImageUrl: normalizeComboImages(combo.dish_image_urls)[0] || '',
        imageCount: normalizeComboImages(combo.dish_image_urls).length,
        submitting: false
      }))

      const nextCombos = reset ? combos : [...this.data.combos, ...combos]
      const total = Number(response.total || 0)

      this.setData({
        combos: nextCombos,
        pageId: pageId + 1,
        hasMore: nextCombos.length < total
      })
    } catch (err) {
      logger.error('Load combos failed', err)
      wx.showToast({ title: '加载套餐失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onPullDownRefresh() {
    this.loadCombos(true)
  },

  onReachBottom() {
    this.loadCombos()
  },

  async onToggleOnline(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const index = this.data.combos.findIndex((combo) => combo.id === id)
    if (index < 0) return

    const current = this.data.combos[index]
    this.setData({ [`combos[${index}].submitting`]: true })

    try {
      const updated = await ComboManagementService.updateComboOnlineStatus(id, {
        is_online: !current.is_online
      })
      this.setData({
        [`combos[${index}].is_online`]: updated.is_online
      })
      wx.showToast({ title: updated.is_online ? '已上架' : '已下架', icon: 'success' })
    } catch (err) {
      logger.error('Toggle combo status failed', err)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      this.setData({ [`combos[${index}].submitting`]: false })
    }
  },

  onEditCombo(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/merchant/combos/edit/index?id=${id}` })
  },

  async onDeleteCombo(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    wx.showModal({
      title: '删除套餐',
      content: '删除后不可恢复，确认删除该套餐吗？',
      success: async (res) => {
        if (!res.confirm) return

        try {
          await ComboManagementService.deleteCombo(id)
          this.setData({
            combos: this.data.combos.filter((combo) => combo.id !== id)
          })
          wx.showToast({ title: '删除成功', icon: 'success' })
        } catch (err) {
          logger.error('Delete combo failed', err)
          wx.showToast({ title: '删除失败', icon: 'none' })
        }
      }
    })
  },

  onCreateCombo() {
    wx.navigateTo({ url: '/pages/merchant/combos/edit/index' })
  }
})
