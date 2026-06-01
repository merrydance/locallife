import { tableManagementService } from '../../../api/table-device-management'
import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import {
  buildTablePresentationState,
  ensureArray,
  formatTableView,
  isPermissionDeniedError,
  isUserCancelledError,
  normalizeQRCodeUrl,
  saveTableQRCodePosterToAlbum,
  TABLE_STATUS_FILTER_OPTIONS,
  type TableListItem,
  type TableStatusFilterKey,
  type TableTypeFilterKey
} from '../_utils/merchant-tables-shared'

interface TableQRCodeContext {
  tableNo: string
  qrCodeUrl: string
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
    loadedTables: [] as TableListItem[],
    tables: [] as TableListItem[],
    tabOptions: [
      { label: '全部 0', value: 'all', count: 0 },
      { label: '普通 0', value: 'table', count: 0 },
      { label: '包间 0', value: 'room', count: 0 }
    ],
    statusFilterOptions: TABLE_STATUS_FILTER_OPTIONS,
    currentTab: 'all' as TableTypeFilterKey,
    currentStatus: 'all' as TableStatusFilterKey,
    resultSummaryText: '当前共 0 项桌台与包间',
    emptyDescription: '还没有桌台或包间，先新增一个',
    qrCodeVisible: false,
    qrCodeLoading: false,
    qrCodeDownloading: false,
    qrCodeError: false,
    qrCodeErrorMessage: '',
    qrCodeImageUrl: '',
    qrCodeTableId: 0,
    qrCodeTableNo: ''
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

    void this.refreshAll()
  },

  buildPresentationUpdate(loadedTables: TableListItem[]) {
    return buildTablePresentationState({
      loadedTables,
      currentTab: this.data.currentTab,
      currentStatus: this.data.currentStatus
    })
  },
  async refreshAll(showLoading = true) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.loading) {
      return
    }

    const hasTrustedList = this.data.loadedTables.length > 0
    this.setData({
      loading: true,
      ...(showLoading || !hasTrustedList
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          }
        : {
            refreshErrorMessage: ''
          })
    })

    try {
      const response = await tableManagementService.listTables()
      const loadedTables = Array.isArray(response?.tables)
        ? response.tables.filter((table): table is Parameters<typeof formatTableView>[0] => !!table).map(formatTableView)
        : []

      this.setData({
        loadedTables,
        ...this.buildPresentationUpdate(loadedTables),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load tables failed', err)
      const message = getErrorUserMessage(err, '桌台列表加载失败，请稍后重试')

      if (!hasTrustedList) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        this.setData({
          initialLoading: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      this.setData({ loading: false, initialLoading: false })
      wx.stopPullDownRefresh()
    }
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

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    void this.refreshAll()
  },

  onRetryRefresh() {
    void this.refreshAll(false)
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    void this.refreshAll(false)
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: TableTypeFilterKey }>) {
    const nextTab = e.detail?.value || 'all'
    if (!nextTab || nextTab === this.data.currentTab) {
      return
    }

    this.setData({
      currentTab: nextTab,
      ...buildTablePresentationState({
        loadedTables: ensureArray(this.data.loadedTables),
        currentTab: nextTab,
        currentStatus: this.data.currentStatus
      })
    })
  },

  onStatusFilterChange(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: TableStatusFilterKey }
    if (!value || value === this.data.currentStatus) {
      return
    }

    this.setData({
      currentStatus: value,
      ...buildTablePresentationState({
        loadedTables: ensureArray(this.data.loadedTables),
        currentTab: this.data.currentTab,
        currentStatus: value
      })
    })
  },

  onAddTable() {
    wx.navigateTo({ url: './edit/index' })
  },

  onEditTable(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    wx.navigateTo({ url: `./edit/index?id=${id}` })
  },

  async onReleaseTable(e: WechatMiniprogram.TouchEvent) {
    const { id, no } = e.currentTarget.dataset as { id?: number, no?: string }
    if (!id) {
      return
    }

    wx.showModal({
      title: '释放桌台',
      content: `确认手动释放 ${no || '当前桌台'} 吗？状态会改为空闲。`,
      confirmText: '确认释放',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        try {
          await tableManagementService.updateTableStatus(id, { status: 'available' })
          await this.refreshAll(false)
        } catch (err) {
          logger.error('Release table failed', err)
          wx.showToast({ title: getErrorUserMessage(err, '释放桌台失败，请稍后重试'), icon: 'none' })
        }
      }
    })
  },

  getTableQRCodeContext(tableId: number): TableQRCodeContext {
    const targetTable = ensureArray(this.data.loadedTables).find((item) => item.id === tableId)
    return {
      tableNo: targetTable?.table_no || '',
      qrCodeUrl: normalizeQRCodeUrl(targetTable?.qrCodeUrl || targetTable?.qr_code_url)
    }
  },

  syncTableQRCodeUrl(tableId: number, qrCodeUrl: string) {
    const nextQrCodeUrl = normalizeQRCodeUrl(qrCodeUrl)
    if (!nextQrCodeUrl) {
      return
    }

    const loadedTables = ensureArray(this.data.loadedTables).map((item) => (
      item.id === tableId
        ? { ...item, qrCodeUrl: nextQrCodeUrl, qr_code_url: nextQrCodeUrl }
        : item
    ))

    this.setData({
      loadedTables,
      ...this.buildPresentationUpdate(loadedTables)
    })
  },

  async fetchTableQRCode(tableId: number, fallbackUrl = '') {
    try {
      const response = await tableManagementService.getTableQRCode(tableId)
      const qrCodeUrl = normalizeQRCodeUrl(response?.qr_code_url)
      if (!qrCodeUrl) {
        throw new Error('missing qr code url')
      }

      this.syncTableQRCodeUrl(tableId, qrCodeUrl)
      this.setData({
        qrCodeLoading: false,
        qrCodeError: false,
        qrCodeErrorMessage: '',
        qrCodeImageUrl: qrCodeUrl
      })
    } catch (err) {
      logger.error('Get table qrcode failed', err)
      if (fallbackUrl) {
        this.setData({
          qrCodeLoading: false,
          qrCodeError: false,
          qrCodeErrorMessage: '',
          qrCodeImageUrl: fallbackUrl
        })
        wx.showToast({ title: '二维码生成失败，已展示当前版本', icon: 'none' })
        return
      }

      this.setData({
        qrCodeLoading: false,
        qrCodeError: true,
        qrCodeErrorMessage: getErrorUserMessage(err, '生成二维码失败，请稍后重试'),
        qrCodeImageUrl: ''
      })
    }
  },

  openTableQRCode(options: { id?: number, tableNo?: string, url?: string, fetchFresh?: boolean } = {}) {
    const tableId = Number(options.id || 0)
    const knownContext = tableId > 0 ? this.getTableQRCodeContext(tableId) : { tableNo: '', qrCodeUrl: '' }
    const fallbackUrl = normalizeQRCodeUrl(options.url) || knownContext.qrCodeUrl
    const tableNo = options.tableNo || knownContext.tableNo
    const shouldFetch = tableId > 0 && (options.fetchFresh || !fallbackUrl)

    if (!tableId && !fallbackUrl) {
      wx.showToast({ title: '暂无二维码', icon: 'none' })
      return
    }

    this.setData({
      qrCodeVisible: true,
      qrCodeLoading: shouldFetch,
      qrCodeDownloading: false,
      qrCodeError: false,
      qrCodeErrorMessage: '',
      qrCodeImageUrl: fallbackUrl,
      qrCodeTableId: tableId,
      qrCodeTableNo: tableNo
    })

    if (shouldFetch) {
      void this.fetchTableQRCode(tableId, fallbackUrl)
    }
  },

  onShowQRCode(e: WechatMiniprogram.TouchEvent) {
    const { id, no, url } = e.currentTarget.dataset as { id?: number, no?: string, url?: string }
    this.openTableQRCode({ id, tableNo: no, url, fetchFresh: !url })
  },

  onCloseQRCodePopup() {
    this.setData({
      qrCodeVisible: false,
      qrCodeLoading: false,
      qrCodeDownloading: false,
      qrCodeError: false,
      qrCodeErrorMessage: ''
    })
  },

  onRetryQRCode() {
    if (!this.data.qrCodeTableId) {
      return
    }

    this.openTableQRCode({
      id: this.data.qrCodeTableId,
      tableNo: this.data.qrCodeTableNo,
      url: this.data.qrCodeImageUrl,
      fetchFresh: true
    })
  },

  async onDownloadQRCode() {
    if (this.data.qrCodeDownloading || !this.data.qrCodeImageUrl) {
      return
    }

    this.setData({ qrCodeDownloading: true })
    wx.showLoading({ title: '保存中...' })

    try {
      await saveTableQRCodePosterToAlbum({
        qrCodeUrl: this.data.qrCodeImageUrl,
        tableNo: this.data.qrCodeTableNo
      })
      wx.showToast({ title: '打印海报已保存到相册', icon: 'success' })
    } catch (err) {
      logger.error('Download table qrcode failed', err)
      if (isPermissionDeniedError(err)) {
        wx.showModal({
          title: '需要相册权限',
          content: '请在设置中开启“保存到相册”权限后重试。',
          confirmText: '去设置',
          success: (result) => {
            if (result.confirm) {
              wx.openSetting()
            }
          }
        })
        return
      }

      if (isUserCancelledError(err)) {
        return
      }

      wx.showToast({ title: '下载二维码失败，请稍后重试', icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ qrCodeDownloading: false })
    }
  }
})
