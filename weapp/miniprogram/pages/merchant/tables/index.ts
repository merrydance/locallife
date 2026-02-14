import { getStableBarHeights } from '../../../utils/responsive'
import { tableManagementService, TableResponse } from '../../../api/table-device-management'
import { logger } from '../../../utils/logger'

type TableTab = 'all' | 'table' | 'room'

interface TableView extends TableResponse {
  statusLabel: string
  statusTheme: string
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    currentTab: 'all' as TableTab,
    tables: [] as TableView[],
    rawTables: [] as TableResponse[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadTables()
  },

  async loadTables() {
    if (this.data.loading) return
    this.setData({ loading: true })
    
    try {
      // API 支持 table_type 过滤，但这里我们先拉取全部在前端切也行，或者根据 tab 传参
      const type = this.data.currentTab === 'all' ? undefined : this.data.currentTab
      const res = await tableManagementService.listTables(type)
      
      const formatted = (res.tables || []).map((t) => this.formatTable(t))
      this.setData({ 
        tables: formatted,
        rawTables: res.tables || []
      })
    } catch (err) {
      logger.error('Load tables failed', err)
      wx.showToast({ title: '加载桌台失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  formatTable(t: TableResponse) {
    const statusMap: Record<string, { label: string, theme: string }> = {
      'available': { label: '空闲', theme: 'success' },
      'occupied': { label: '就餐中', theme: 'error' },
      'reserved': { label: '已预订', theme: 'warning' },
      'disabled': { label: '停用', theme: 'default' }
    }
    const statusInfo = statusMap[t.status] || { label: t.status, theme: 'default' }
    
    return {
      ...t,
      statusLabel: statusInfo.label,
      statusTheme: statusInfo.theme
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: TableTab }>) {
    this.setData({ currentTab: e.detail.value || 'all' }, () => {
      this.loadTables()
    })
  },

  onPullDownRefresh() {
    this.loadTables()
  },

  async onReleaseTable(e: WechatMiniprogram.TouchEvent) {
    const { id, no } = e.currentTarget.dataset as { id?: number, no?: string }
    if (!id) return

    wx.showModal({
      title: '释放确认',
      content: `确认手动释放桌台 ${no} 吗？这将其状态改为“空闲”。`,
      confirmText: '确认释放',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm) return
        try {
          await tableManagementService.updateTableStatus(id, { status: 'available' })
          wx.showToast({ title: '已释放', icon: 'success' })
          this.loadTables()
        } catch (err) {
          logger.error('Release table failed', err)
          wx.showToast({ title: '操作失败', icon: 'none' })
        }
      }
    })
  },

  onShowQRCode(e: WechatMiniprogram.TouchEvent) {
    const { url } = e.currentTarget.dataset as { url?: string }
    if (!url) {
      return wx.showToast({ title: '暂无二维码', icon: 'none' })
    }
    wx.previewImage({
      urls: [url],
      current: url
    })
  },

  onAddTable() {
    wx.showToast({ title: '跳转新增桌台', icon: 'none' })
    // wx.navigateTo({ url: './edit/index' })
  },

  onTableClick(_e: WechatMiniprogram.TouchEvent) {
    // const { id } = e.currentTarget.dataset
    // wx.navigateTo({ url: `./detail/index?id=${id}` })
  }
})
