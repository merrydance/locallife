import { getTableStatusDisplay } from '../../../api/table'
import {
  getDiningSessionEntry,
  getDiningSessionMenu,
  openDiningSession,
  transferDiningSessionTable,
  type DiningSessionEntryResponse,
  type DiningSessionEntrySessionSummary,
  type DiningSessionPrecheckResponse
} from '../../../api/dining-session'
import {
  saveDineInSessionFromEntrySummary,
  saveDineInSessionFromMenu,
  saveDineInSessionFromOpenResponse
} from '../../../services/dine-in-session'
import { getErrorUserMessage } from '../../../utils/user-facing'

type EntryParams = {
  merchant_id?: number
  table_no?: string
  table_id?: number
}

function parseScene(scene: string): EntryParams | null {
  const decoded = decodeURIComponent(scene)
  const merchantMatch = decoded.match(/m_(\d+)/)
  const tableNoMatch = decoded.match(/t_([^-]+)/)
  const tableIdMatch = decoded.match(/tid_(\d+)/)

  if (merchantMatch && tableNoMatch) {
    return {
      merchant_id: parseInt(merchantMatch[1], 10),
      table_no: tableNoMatch[1]
    }
  }

  if (tableIdMatch) {
    return { table_id: parseInt(tableIdMatch[1], 10) }
  }

  const legacyTableId = decoded.replace(/^(table_|t)/, '')
  if (legacyTableId && !Number.isNaN(parseInt(legacyTableId, 10))) {
    return { table_id: parseInt(legacyTableId, 10) }
  }

  return null
}

function parseQrUrl(url: string): EntryParams | null {
  const urlObj = new URL(url)
  const tableId = urlObj.searchParams.get('table_id')
  if (tableId && !Number.isNaN(parseInt(tableId, 10))) {
    return { table_id: parseInt(tableId, 10) }
  }

  const merchantId = urlObj.searchParams.get('merchant_id')
  const tableNo = urlObj.searchParams.get('table_no')
  if (merchantId && tableNo && !Number.isNaN(parseInt(merchantId, 10))) {
    return {
      merchant_id: parseInt(merchantId, 10),
      table_no: tableNo
    }
  }

  const pathTableId = urlObj.pathname.match(/\/table\/(\d+)/)?.[1]
  if (pathTableId && !Number.isNaN(parseInt(pathTableId, 10))) {
    return { table_id: parseInt(pathTableId, 10) }
  }

  return null
}

Page({
  data: {
    loading: true,
    error: '',
    navBarHeight: 88,
    merchantInfo: null as DiningSessionEntryResponse['merchant'] | null,
    tableInfo: null as DiningSessionEntryResponse['table'] | null,
    precheck: null as DiningSessionPrecheckResponse | null,
    activeSession: null as DiningSessionEntrySessionSummary | null,
    transferSession: null as DiningSessionEntrySessionSummary | null,
    action: 'blocked' as DiningSessionEntryResponse['action'],
    blockedReason: '',
    requiresTableCode: false,
    transferRequiresTableCode: false,
    statusLabel: '',
    statusBadgeClass: '',
    tableCode: '',
    transferCode: '',
    submitting: false,
    showTransferDialog: false,
    primaryActionLabel: '开始点餐',
    entryParams: null as EntryParams | null
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onLoad(options: { scene?: string, q?: string, table_id?: string, merchant_id?: string, table_no?: string }) {
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })

    const entryParams = this.resolveEntryParams(options)
    if (!entryParams) {
      this.setData({ loading: false, error: '无效的扫码参数' })
      return
    }

    this.loadEntry(entryParams)
  },

  resolveEntryParams(options: { scene?: string, q?: string, table_id?: string, merchant_id?: string, table_no?: string }): EntryParams | null {
    if (options.scene) {
      return parseScene(options.scene)
    }
    if (options.q) {
      try {
        return parseQrUrl(decodeURIComponent(options.q))
      } catch (_error) {
        return null
      }
    }
    if (options.table_id) {
      const tableId = parseInt(options.table_id, 10)
      if (!Number.isNaN(tableId)) {
        return { table_id: tableId }
      }
    }
    if (options.merchant_id && options.table_no) {
      const merchantId = parseInt(options.merchant_id, 10)
      if (!Number.isNaN(merchantId)) {
        return { merchant_id: merchantId, table_no: options.table_no }
      }
    }
    return null
  },

  async loadEntry(entryParams: EntryParams) {
    this.setData({ loading: true, error: '', entryParams })
    try {
      const response = await getDiningSessionEntry(entryParams)
      const statusDisplay = getTableStatusDisplay(response.table.status)
      this.setData({
        loading: false,
        merchantInfo: response.merchant,
        tableInfo: response.table,
        precheck: response.precheck,
        activeSession: response.active_session || null,
        transferSession: response.transfer_session || null,
        action: response.action,
        blockedReason: response.blocked_reason || '',
        requiresTableCode: response.capabilities.requires_table_code,
        transferRequiresTableCode: response.capabilities.transfer_requires_table_code,
        statusLabel: statusDisplay.label,
        statusBadgeClass: statusDisplay.badgeClass,
        primaryActionLabel: this.getPrimaryActionLabel(response.action)
      })
    } catch (error) {
      this.setData({
        loading: false,
        error: getErrorUserMessage(error, '获取桌台信息失败，请稍后重试')
      })
    }
  },

  getPrimaryActionLabel(action: DiningSessionEntryResponse['action']) {
    switch (action) {
      case 'resume_session':
        return '继续点餐'
      case 'transfer_session':
        return '换到当前桌并点餐'
      case 'blocked':
        return '当前不可点餐'
      default:
        return '开始点餐'
    }
  },

  onTableCodeInput(e: WechatMiniprogram.Input) {
    this.setData({ tableCode: e.detail.value })
  },

  onTransferCodeInput(e: WechatMiniprogram.Input) {
    this.setData({ transferCode: e.detail.value })
  },

  async onPrimaryAction() {
    const { action, activeSession, merchantInfo, tableInfo, precheck, requiresTableCode, tableCode, submitting } = this.data
    if (submitting || !merchantInfo || !tableInfo) {
      return
    }

    if (action === 'blocked') {
      return
    }

    if (action === 'resume_session' && activeSession) {
      saveDineInSessionFromEntrySummary(activeSession, merchantInfo, tableInfo)
      this.navigateToMenu(activeSession.session.id)
      return
    }

    if (action === 'transfer_session') {
      this.setData({ showTransferDialog: true, transferCode: '' })
      return
    }

    if (requiresTableCode && (!tableCode || tableCode.trim() === '')) {
      wx.showToast({ title: '请输入桌台验证码', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      const response = await openDiningSession({
        table_id: tableInfo.id,
        reservation_id: precheck?.reserved && precheck.is_reservation_owner ? precheck.reservation_id : undefined,
        table_code: precheck?.reserved && precheck.is_reservation_owner ? undefined : tableCode.trim() || undefined
      })
      saveDineInSessionFromOpenResponse(response, merchantInfo, tableInfo)
      this.navigateToMenu(response.session.id)
    } catch (error) {
      wx.showToast({ title: getErrorUserMessage(error, '开台失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  async confirmTransfer() {
    const { transferSession, tableInfo, transferCode, transferRequiresTableCode, merchantInfo, showTransferDialog, submitting } = this.data
    if (!showTransferDialog || submitting || !transferSession || !tableInfo || !merchantInfo) {
      return
    }

    if (transferRequiresTableCode && (!transferCode || transferCode.trim() === '')) {
      wx.showToast({ title: '请输入当前桌台验证码', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      await transferDiningSessionTable(transferSession.session.id, {
        to_table_id: tableInfo.id,
        table_code: transferRequiresTableCode ? transferCode.trim() : undefined,
        reason: '扫码换桌'
      })
      const menuResponse = await getDiningSessionMenu(transferSession.session.id)
      saveDineInSessionFromMenu(menuResponse)
      this.setData({ showTransferDialog: false, submitting: false, transferCode: '' })
      this.navigateToMenu(transferSession.session.id)
    } catch (error) {
      wx.showToast({ title: getErrorUserMessage(error, '换桌失败，请稍后重试'), icon: 'none' })
      this.setData({ submitting: false })
    }
  },

  cancelTransfer() {
    this.setData({ showTransferDialog: false, transferCode: '' })
  },

  navigateToMenu(sessionId: number) {
    wx.navigateTo({ url: `/pages/dine-in/menu/menu?session_id=${sessionId}` })
  },

  onRetry() {
    if (this.data.entryParams) {
      this.loadEntry(this.data.entryParams)
    }
  },

  onShareAppMessage() {
    const { tableInfo, merchantInfo } = this.data
    return {
      title: `${merchantInfo?.name || '餐厅'}的${tableInfo?.table_no || ''}号桌`,
      path: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableInfo?.id || ''}`,
      imageUrl: merchantInfo?.logo_url
    }
  },

  onShareTimeline() {
    const { tableInfo, merchantInfo } = this.data
    return {
      title: `在${merchantInfo?.name || '餐厅'}用餐`,
      query: `table_id=${tableInfo?.id || ''}`,
      imageUrl: merchantInfo?.logo_url
    }
  }
})