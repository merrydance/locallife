import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import { getStableBarHeights } from '../../../utils/responsive'
import {
  cancelReservation,
  checkInReservation,
  completeReservationByMerchant,
  confirmReservationByMerchant,
  markReservationNoShow,
  ReservationService,
  startCookingReservation
} from '../_main_shared/api/reservation'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import {
  ensureMerchantConsoleAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../utils/console-access'
import {
  buildListSummaryText,
  buildReservationCard,
  buildReservationEditUrl,
  buildReservationMoreActionItems,
  buildReservationWorkbenchResetPatch,
  DEFAULT_TAB_OPTIONS,
  getReservationActionDialogConfig,
  getReservationActionLoadingText,
  getReservationActionSuccessText,
  getReservationListStatusFilter,
  LoadReservationListOptions,
  PAGE_AUTO_REFRESH_WINDOW_MS,
  RefreshPageOptions,
  RESERVATION_CANCEL_REASONS,
  type ReservationActionSheetItem,
  type ReservationActionSheetMode,
  type ReservationCardView,
  type ReservationMutationKey,
  type ReservationPrimaryActionKey,
  type ReservationWorkbenchTab
} from '../_utils/merchant-reservations-view'

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    pageSyncing: false,
    lastLoadedAt: 0,
    date: '',
    datePickerVisible: false,
    currentTab: 'all' as ReservationWorkbenchTab,
    tabOptions: DEFAULT_TAB_OPTIONS,
    listAvailable: false,
    listInitialError: false,
    listInitialErrorMessage: '',
    listRefreshErrorMessage: '',
    listLoading: false,
    reservations: [] as ReservationCardView[],
    listPage: 1,
    listPageSize: 20,
    listHasMore: true,
    listTotal: 0,
    listSummaryText: '当前共 0 条预订',
    actionSubmittingKey: '',
    confirmDialogVisible: false,
    confirmDialogTitle: '',
    confirmDialogContent: '',
    confirmDialogConfirmText: '确认',
    confirmDialogConfirmTheme: 'primary',
    confirmDialogSubmitting: false,
    confirmDialogAction: '' as ReservationMutationKey | '',
    confirmDialogReservationId: 0,
    confirmDialogContact: '',
    confirmDialogCancelReason: '',
    actionSheetVisible: false,
    actionSheetMode: '' as ReservationActionSheetMode,
    actionSheetDescription: '',
    actionSheetItems: [] as ReservationActionSheetItem[],
    actionSheetReservationId: 0,
    actionSheetReservationContact: ''
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      date: dayjs().format('YYYY-MM-DD')
    })
    await this.initializePage()
  },

  async onShow() {
    if (
      !this.data.accessReady ||
      this.data.accessDenied ||
      this.data.accessErrorMessage ||
      this.data.initialLoading ||
      this.data.pageSyncing ||
      this.data.listLoading
    ) {
      return
    }

    if (!this.data.lastLoadedAt || Date.now() - this.data.lastLoadedAt >= PAGE_AUTO_REFRESH_WINDOW_MS) {
      await this.refreshPage({
        showLoading: false,
        preserveList: this.data.reservations.length > 0
      })
    }
  },

  async initializePage() {
    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: isMerchantConsoleAccessDenied(accessResult),
      accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult)
    })

    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({ initialLoading: false, pageSyncing: false })
      wx.stopPullDownRefresh()
      return
    }

    await this.refreshPage({ showLoading: true, preserveList: false })
  },

  async refreshPage(options?: RefreshPageOptions) {
    if (this.data.pageSyncing) return

    const showLoading = options?.showLoading !== false
    const preserveList = !!options?.preserveList
    const hasTrustedData = this.data.listAvailable

    this.setData({
      pageSyncing: true,
      ...(showLoading && !hasTrustedData
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: ''
          }
        : {})
    })

    const listOk = await this.loadReservationList(true, {
      showLoading,
      preserveCurrent: preserveList
    })

    this.setData({
      pageSyncing: false,
      initialLoading: false,
      ...(listOk
        ? {
            initialError: false,
            initialErrorMessage: '',
            lastLoadedAt: Date.now()
          }
        : !hasTrustedData
          ? {
              initialError: true,
              initialErrorMessage: '预订列表加载失败，请重试'
            }
          : {})
    })

    wx.stopPullDownRefresh()
  },

  async loadReservationList(reset = false, options?: LoadReservationListOptions) {
    if (this.data.listLoading) return false
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return false
    if (!reset && !this.data.listHasMore) return false

    const showLoading = options?.showLoading !== false
    const hasExistingReservations = this.data.reservations.length > 0
    const preserveCurrent = !!options?.preserveCurrent && reset && hasExistingReservations
    const shouldToggleLoading = !reset || showLoading || !preserveCurrent
    const page = reset ? 1 : this.data.listPage

    this.setData({
      ...(shouldToggleLoading ? { listLoading: true } : {}),
      ...(showLoading
        ? {
            listInitialError: false,
            listInitialErrorMessage: '',
            listRefreshErrorMessage: ''
          }
        : preserveCurrent
          ? { listRefreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await ReservationService.getMerchantReservations({
        page_id: page,
        page_size: this.data.listPageSize,
        date: this.data.date,
        status: getReservationListStatusFilter(this.data.currentTab)
      })

      const nextReservations = Array.isArray(response.reservations)
        ? response.reservations.map(buildReservationCard)
        : []
      const total = typeof response.total === 'number' ? response.total : nextReservations.length
      const reservations = reset ? nextReservations : [...this.data.reservations, ...nextReservations]

      this.setData({
        reservations,
        listAvailable: true,
        listInitialError: false,
        listInitialErrorMessage: '',
        listRefreshErrorMessage: '',
        listPage: page + 1,
        listHasMore: page * this.data.listPageSize < total,
        listTotal: total,
        listSummaryText: buildListSummaryText(this.data.currentTab, total)
      })
      return true
    } catch (err) {
      logger.error('Load merchant reservation list failed', err)
      const message = getErrorMessage(err, '预订列表加载失败，请稍后重试')

      if (preserveCurrent || hasExistingReservations) {
        this.setData({ listRefreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        this.setData({
          reservations: [],
          listAvailable: false,
          listInitialError: true,
          listInitialErrorMessage: message,
          listPage: 1,
          listHasMore: true,
          listTotal: 0,
          listSummaryText: buildListSummaryText(this.data.currentTab, 0)
        })
      }
      return false
    } finally {
      if (shouldToggleLoading) {
        this.setData({ listLoading: false })
      }
      wx.stopPullDownRefresh()
    }
  },

  onOpenDatePicker() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.pageSyncing || this.data.listLoading) return
    this.setData({ datePickerVisible: true })
  },

  onCloseDatePicker() { this.setData({ datePickerVisible: false }) },

  applyDateSelection(nextDate: string) {
    if (!nextDate || nextDate === this.data.date) {
      this.setData({ datePickerVisible: false })
      return
    }

    this.setData({
      date: nextDate,
      datePickerVisible: false
    })

    void this.refreshPage({
      showLoading: false,
      preserveList: false
    })
  },

  onDateConfirm(e: WechatMiniprogram.CustomEvent<{ value: string }>) { this.applyDateSelection(e.detail?.value || '') },

  async onTabChange(e: WechatMiniprogram.CustomEvent<{ value: ReservationWorkbenchTab }>) {
    const nextTab = e.detail.value
    if (!nextTab || nextTab === this.data.currentTab || this.data.listLoading) return

    const previousTab = this.data.currentTab
    this.setData({ currentTab: nextTab })

    const success = await this.loadReservationList(true, {
      showLoading: false,
      preserveCurrent: this.data.reservations.length > 0
    })

    if (!success) {
      this.setData({
        currentTab: previousTab,
        listSummaryText: buildListSummaryText(previousTab, this.data.listTotal)
      })
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }
    void this.refreshPage({ showLoading: false, preserveList: this.data.reservations.length > 0 })
  },

  onReachBottom() { void this.loadReservationList() },
  onLoadMore() { void this.loadReservationList() },

  onRetry() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }
    void this.refreshPage({ showLoading: true, preserveList: false })
  },

  onManualRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      void this.onRetryAccess()
      return
    }
    void this.refreshPage({ showLoading: false, preserveList: this.data.reservations.length > 0 })
  },

  onRetryList() { void this.loadReservationList(true, { showLoading: true, preserveCurrent: false }) },

  async onRetryAccess() {
    this.setData(buildReservationWorkbenchResetPatch(this.data.date))

    await this.initializePage()
  },

  onActionsCatch() {},

  getReservationCard(id: number) { return this.data.reservations.find((item) => item.id === id) },

  openReservationEditPage(id?: number) {
    wx.navigateTo({ url: buildReservationEditUrl(this.data.date, id) })
  },

  onOpenCreatePage() { this.openReservationEditPage() },

  onReservationCardTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (id) this.openReservationEditPage(id)
  },
  onOpenEditPage(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (id) this.openReservationEditPage(id)
  },

  showFeedbackToast(theme: 'success' | 'warning' | 'error', message: string, duration = 2200) {
    wx.showToast({
      title: message,
      icon: theme === 'success' ? 'success' : 'none',
      duration
    })
  },

  showLoadingToast(message: string) {
    wx.showLoading({ title: message, mask: true })
  },

  hideLoadingToast() {
    wx.hideLoading()
  },

  openReservationActionSheet(
    mode: ReservationActionSheetMode,
    reservationId: number,
    contact: string,
    description: string,
    items: ReservationActionSheetItem[]
  ) {
    if (!reservationId || !items.length) return

    this.setData({
      actionSheetVisible: true,
      actionSheetMode: mode,
      actionSheetDescription: description,
      actionSheetItems: items,
      actionSheetReservationId: reservationId,
      actionSheetReservationContact: contact || ''
    })
  },

  resetReservationActionSheet() {
    this.setData({
      actionSheetVisible: false,
      actionSheetMode: '',
      actionSheetDescription: '',
      actionSheetItems: [],
      actionSheetReservationId: 0,
      actionSheetReservationContact: ''
    })
  },

  openCancelReasonSheet(reservationId: number, contact?: string) {
    this.openReservationActionSheet(
      'cancel_reasons',
      reservationId,
      contact || '',
      '请选择取消原因',
      RESERVATION_CANCEL_REASONS.map((reason) => ({
        label: reason,
        key: 'cancel_reason',
        reason,
        color: 'default'
      }))
    )
  },

  onActionSheetCancel() { this.resetReservationActionSheet() },
  onActionSheetClose() { this.resetReservationActionSheet() },

  onActionSheetSelected(
    e: WechatMiniprogram.CustomEvent<{ selected?: ReservationActionSheetItem | string, index?: number }>
  ) {
    const selected = e.detail?.selected
    const selectedItem = typeof selected === 'string' ? undefined : selected
    const selectedKey = typeof selected === 'string' ? selected : selected?.key
    const mode = this.data.actionSheetMode
    const reservationId = this.data.actionSheetReservationId
    const contact = this.data.actionSheetReservationContact

    this.resetReservationActionSheet()

    if (!reservationId || !selectedKey) {
      return
    }

    if (mode === 'actions') {
      if (selectedKey === 'edit') {
        this.openReservationEditPage(reservationId)
        return
      }

      if (selectedKey === 'cancel') {
        this.openCancelReasonSheet(reservationId, contact)
        return
      }

      void this.triggerReservationActionByKey(reservationId, selectedKey as ReservationMutationKey, contact)
      return
    }

    if (mode === 'cancel_reasons' && selectedKey === 'cancel_reason' && selectedItem?.reason) {
      this.openConfirmDialog('cancel', reservationId, contact, selectedItem.reason)
    }
  },

  openConfirmDialog(
    actionKey: ReservationMutationKey,
    reservationId: number,
    contact?: string,
    cancelReason?: string
  ) {
    const dialog = getReservationActionDialogConfig(actionKey, contact, cancelReason)

    this.setData({
      confirmDialogVisible: true,
      confirmDialogTitle: dialog.title,
      confirmDialogContent: dialog.content,
      confirmDialogConfirmText: dialog.confirmText,
      confirmDialogConfirmTheme: dialog.confirmTheme,
      confirmDialogSubmitting: false,
      confirmDialogAction: actionKey,
      confirmDialogReservationId: reservationId,
      confirmDialogContact: contact || '',
      confirmDialogCancelReason: cancelReason || ''
    })
  },

  resetConfirmDialogState() {
    this.setData({
      confirmDialogVisible: false,
      confirmDialogTitle: '',
      confirmDialogContent: '',
      confirmDialogConfirmText: '确认',
      confirmDialogConfirmTheme: 'primary',
      confirmDialogSubmitting: false,
      confirmDialogAction: '',
      confirmDialogReservationId: 0,
      confirmDialogContact: '',
      confirmDialogCancelReason: ''
    })
  },

  onCancelConfirmDialog() { if (!this.data.confirmDialogSubmitting) this.resetConfirmDialogState() },

  async executeReservationAction(
    reservationId: number,
    actionKey: ReservationMutationKey,
    cancelReason?: string
  ) {
    switch (actionKey) {
      case 'confirm':
        return confirmReservationByMerchant(reservationId)
      case 'check_in':
        return checkInReservation(reservationId)
      case 'start_cooking':
        return startCookingReservation(reservationId)
      case 'complete':
        return completeReservationByMerchant(reservationId)
      case 'no_show':
        return markReservationNoShow(reservationId)
      case 'cancel':
        return cancelReservation(reservationId, cancelReason)
      default:
        return undefined
    }
  },

  async onConfirmDialogAction() {
    const reservationId = Number(this.data.confirmDialogReservationId || 0)
    const actionKey = this.data.confirmDialogAction as ReservationMutationKey | ''
    const cancelReason = this.data.confirmDialogCancelReason || ''

    if (!reservationId || !actionKey || this.data.confirmDialogSubmitting) {
      this.onCancelConfirmDialog()
      return
    }

    const submittingKey = `${actionKey}:${reservationId}`
    let loadingToastVisible = false

    this.setData({
      confirmDialogSubmitting: true,
      actionSubmittingKey: submittingKey
    })
    this.showLoadingToast(getReservationActionLoadingText(actionKey))
    loadingToastVisible = true

    try {
      await this.executeReservationAction(reservationId, actionKey, cancelReason)
      if (loadingToastVisible) {
        this.hideLoadingToast()
        loadingToastVisible = false
      }

      this.resetConfirmDialogState()
      await this.refreshPage({
        showLoading: false,
        preserveList: this.data.reservations.length > 0
      })
      this.showFeedbackToast('success', getReservationActionSuccessText(actionKey))
    } catch (err) {
      logger.error(`Reservation action failed: ${actionKey}`, err)
      if (loadingToastVisible) {
        this.hideLoadingToast()
        loadingToastVisible = false
      }

      this.resetConfirmDialogState()
      if (actionKey === 'check_in') {
        await this.refreshPage({
          showLoading: false,
          preserveList: this.data.reservations.length > 0
        })
      }

      this.showFeedbackToast('warning', getErrorMessage(err, '操作失败'))
    } finally {
      if (loadingToastVisible) {
        this.hideLoadingToast()
      }
      this.setData({
        confirmDialogSubmitting: false,
        actionSubmittingKey: ''
      })
    }
  },

  async triggerReservationActionByKey(reservationId: number, actionKey: ReservationMutationKey, contact?: string) {
    if (!reservationId) return

    switch (actionKey) {
      case 'confirm':
        this.openConfirmDialog('confirm', reservationId, contact)
        return
      case 'check_in':
        this.openConfirmDialog('check_in', reservationId, contact)
        return
      case 'start_cooking':
        this.openConfirmDialog('start_cooking', reservationId, contact)
        return
      case 'complete':
        this.openConfirmDialog('complete', reservationId, contact)
        return
      case 'no_show':
        this.openConfirmDialog('no_show', reservationId, contact)
        return
      case 'cancel':
        this.openCancelReasonSheet(reservationId, contact)
        return
      default:
        return
    }
  },

  onPrimaryAction(e: WechatMiniprogram.TouchEvent) {
    const { id, action, contact } = e.currentTarget.dataset as {
      id?: number
      action?: ReservationPrimaryActionKey
      contact?: string
    }

    if (!id || !action) return
    void this.triggerReservationActionByKey(id, action, contact)
  },

  onOpenMoreActions(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const reservation = this.getReservationCard(id)
    if (!reservation) return

    const optionDefs = buildReservationMoreActionItems(reservation)
    if (!optionDefs.length) return

    this.openReservationActionSheet(
      'actions',
      id,
      reservation.contact_name,
      `${reservation.contact_name || '当前顾客'} 的预订操作`,
      optionDefs
    )
  }
})