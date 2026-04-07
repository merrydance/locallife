import { getStableBarHeights } from '../../../utils/responsive'
import { tableManagementService, CreateTableRequest, TableImageResponse, TableResponse, UpdateTableRequest } from '../../../api/table-device-management'
import { getTableStatusDisplay, TableStatus } from '../../../api/table'
import { TagService } from '../../../api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'

type TableTab = 'all' | 'table' | 'room'

type TableLoadSource = 'initial' | 'retry' | 'refresh' | 'tab'

interface TableView extends TableResponse {
  statusLabel: string
  statusTheme: string
  canRelease: boolean
  canShowCode: boolean
}

interface TableStatusSelection {
  available: boolean
  occupied: boolean
  reserved: boolean
  disabled: boolean
}

interface TableTagOption {
  id: number
  name: string
}

interface TableUploadFile {
  url: string
  status?: 'loading' | 'done' | 'failed'
  mediaId?: number
  localPath?: string
}

interface TableFormData {
  table_no: string
  table_type: 'table' | 'room'
  capacity: number
  description: string
  minimum_spend_yuan: string
  status: TableStatus
  access_code: string
  tag_ids: number[]
}

const getErrorMessage = getErrorUserMessage

function createDefaultFormData(): TableFormData {
  return {
    table_no: '',
    table_type: 'table',
    capacity: 4,
    description: '',
    minimum_spend_yuan: '',
    status: 'available',
    access_code: '',
    tag_ids: []
  }
}

function buildTableStatusSelection(status?: TableStatus | string): TableStatusSelection {
  const normalizedStatus = getTableStatusDisplay(status).normalizedStatus
  const selection: TableStatusSelection = {
    available: false,
    occupied: false,
    reserved: false,
    disabled: false
  }

  if (normalizedStatus in selection) {
    selection[normalizedStatus as keyof TableStatusSelection] = true
  }

  return selection
}

function buildTableTabLabel(tab: TableTab): string {
  if (tab === 'table') return '普通'
  if (tab === 'room') return '包间'
  return '全部'
}

function normalizeQRCodeUrl(path?: string): string {
  if (!path) return ''
  return getPublicImageUrl(path)
}

interface TableQRCodeContext {
  tableNo: string
  qrCodeUrl: string
}

function getMiniProgramErrorMessage(error: unknown): string {
  if (typeof error === 'string') return error
  if (error && typeof error === 'object' && typeof (error as { errMsg?: unknown }).errMsg === 'string') {
    return (error as { errMsg: string }).errMsg
  }
  if (error instanceof Error) return error.message
  return ''
}

function isPermissionDeniedError(error: unknown): boolean {
  const message = getMiniProgramErrorMessage(error)
  return message.includes('auth deny') || message.includes('auth denied')
}

function isUserCancelledError(error: unknown): boolean {
  return getMiniProgramErrorMessage(error).includes('cancel')
}

function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

function toSafeTagOptions(value: unknown): TableTagOption[] {
  if (!Array.isArray(value)) return []
  const result: TableTagOption[] = []
  for (const item of value) {
    if (!item || typeof item !== 'object') continue
    const candidate = item as { id?: unknown, name?: unknown }
    if (typeof candidate.id !== 'number' || candidate.id <= 0) continue
    result.push({ id: candidate.id, name: typeof candidate.name === 'string' ? candidate.name : '' })
  }
  return result
}

function toSafeTableImages(value: unknown): TableImageResponse[] {
  if (!Array.isArray(value)) return []
  const result: TableImageResponse[] = []
  for (const item of value) {
    if (!item || typeof item !== 'object') continue
    const candidate = item as TableImageResponse
    const normalizedImageUrl = getPublicImageUrl(typeof candidate.image_url === 'string' ? candidate.image_url : '')
    if (!normalizedImageUrl) continue

    result.push({
      id: typeof candidate.id === 'number' ? candidate.id : undefined,
      table_id: typeof candidate.table_id === 'number' ? candidate.table_id : undefined,
      image_url: normalizedImageUrl,
      sort_order: typeof candidate.sort_order === 'number' ? candidate.sort_order : undefined,
      is_primary: !!candidate.is_primary
    })
  }
  return result
}

function toSafeUploadFiles(value: unknown): TableUploadFile[] {
  if (!Array.isArray(value)) return []
  const result: TableUploadFile[] = []
  for (const item of value) {
    if (!item || typeof item !== 'object') continue
    const candidate = item as TableUploadFile
    if (typeof candidate.url !== 'string' || !candidate.url) continue
    result.push({
      url: candidate.url,
      status: candidate.status,
      mediaId: typeof candidate.mediaId === 'number' ? candidate.mediaId : undefined,
      localPath: typeof candidate.localPath === 'string' ? candidate.localPath : undefined
    })
  }
  return result
}

function findUploadFileIndex(files: TableUploadFile[], localPath: string): number {
  return files.findIndex((file) => file.localPath === localPath)
}

function replaceUploadFileAt(files: TableUploadFile[], index: number, file: TableUploadFile): TableUploadFile[] {
  if (index < 0 || index >= files.length) {
    return files
  }

  const next = [...files]
  next[index] = file
  return next
}

function removeUploadFileAt(files: TableUploadFile[], index: number): TableUploadFile[] {
  if (index < 0 || index >= files.length) {
    return files
  }

  const next = [...files]
  next.splice(index, 1)
  return next
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    loading: false,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    syncMessage: '',
    listLoaded: false,
    formSubmitting: false,
    currentTab: 'all' as TableTab,
    loadedTab: 'all' as TableTab,
    tables: [] as TableView[],
    rawTables: [] as TableResponse[],
    availableTags: [] as TableTagOption[],
    tableImages: [] as TableImageResponse[],
    uploadFiles: [] as TableUploadFile[],
    pendingMediaIds: [] as number[],
    pendingImagePreviews: [] as string[],
    imageUploading: false,
    imageMutating: false,
    imageMutatingImageId: 0,
    tagSubmitting: false,
    qrCodeVisible: false,
    qrCodeLoading: false,
    qrCodeDownloading: false,
    qrCodeError: false,
    qrCodeErrorMessage: '',
    qrCodeImageUrl: '',
    qrCodeTableId: 0,
    qrCodeTableNo: '',
    formVisible: false,
    isEdit: false,
    editingTableId: 0,
    formData: createDefaultFormData(),
    formStatusSelection: buildTableStatusSelection('available') as TableStatusSelection
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.normalizeListState()

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

    this.loadTables({ source: 'initial' })
    this.loadAvailableTags()
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

  onShow() {
    this.normalizeListState()
  },

  normalizeListState() {
    this.setData({
      tables: ensureArray(this.data.tables),
      rawTables: ensureArray(this.data.rawTables),
      availableTags: ensureArray(this.data.availableTags),
      tableImages: ensureArray(this.data.tableImages),
      uploadFiles: toSafeUploadFiles(this.data.uploadFiles),
      pendingMediaIds: ensureArray(this.data.pendingMediaIds),
      pendingImagePreviews: ensureArray(this.data.pendingImagePreviews),
      'formData.tag_ids': ensureArray(this.data.formData?.tag_ids)
    })
  },

  async loadAvailableTags() {
    try {
      const tags = await TagService.listTags('table')
      this.setData({
        availableTags: toSafeTagOptions(tags)
      })
    } catch (err) {
      logger.error('Load table tags failed', err)
      this.setData({ availableTags: [] })
    }
  },

  async loadTables(options: { requestedTab?: TableTab, source?: TableLoadSource } = {}) {
    if (this.data.loading) return

    const requestedTab = options.requestedTab || this.data.currentTab
    const source = options.source || 'refresh'
    const hasTrustedList = this.data.listLoaded
    const fallbackTab = this.data.loadedTab || this.data.currentTab || 'all'
    const isTabSwitch = source === 'tab' && requestedTab !== fallbackTab

    this.setData({
      loading: true,
      ...(hasTrustedList
        ? {
            refreshErrorMessage: '',
            syncMessage: isTabSwitch
              ? `正在同步${buildTableTabLabel(requestedTab)}桌台...`
              : '正在同步桌台列表...'
          }
        : {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: '',
            syncMessage: ''
          })
    })
    
    try {
      const type = requestedTab === 'all' ? undefined : requestedTab
      const res = await tableManagementService.listTables(type)
      
      const rawTables = Array.isArray(res?.tables)
        ? res.tables.filter((table): table is TableResponse => !!table && typeof table === 'object')
        : []

      const formatted = rawTables.map((t) => this.formatTable(t))
      this.setData({ 
        currentTab: requestedTab,
        loadedTab: requestedTab,
        tables: formatted,
        rawTables,
        listLoaded: true,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        syncMessage: ''
      })
    } catch (err) {
      logger.error('Load tables failed', err)
      const message = getErrorMessage(err, '加载桌台失败，请稍后重试')
      if (!hasTrustedList) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          syncMessage: ''
        })
      } else {
        this.setData({
          currentTab: fallbackTab,
          refreshErrorMessage: isTabSwitch
            ? `${message}，当前仍显示${buildTableTabLabel(fallbackTab)}桌台`
            : `${message}，当前已保留上次同步结果`,
          syncMessage: ''
        })
      }
    } finally {
      this.setData({ loading: false, initialLoading: false, syncMessage: '' })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadTables({ source: 'retry' })
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadTables({ source: 'refresh' })
  },

  formatTable(t: TableResponse) {
    const statusInfo = getTableStatusDisplay(t.status)

    return {
      ...t,
      status: statusInfo.normalizedStatus,
      statusLabel: statusInfo.label,
      statusTheme: statusInfo.theme,
      canRelease: statusInfo.canRelease,
      canShowCode: statusInfo.canShowCode
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: TableTab }>) {
    const nextTab = e.detail.value || 'all'
    const activeTab = this.data.loadedTab || this.data.currentTab || 'all'
    if (!nextTab || nextTab === activeTab) return

    if (this.data.loading) {
      wx.showToast({ title: '正在同步列表，请稍后再试', icon: 'none' })
      return
    }

    this.loadTables({ requestedTab: nextTab, source: 'tab' })
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadTables({ source: 'refresh' })
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
          this.loadTables()
        } catch (err) {
          logger.error('Release table failed', err)
          wx.showToast({ title: '操作失败', icon: 'none' })
        }
      }
    })
  },

  getTableQRCodeContext(tableId: number): TableQRCodeContext {
    const rawTable = ensureArray(this.data.rawTables).find((item) => item.id === tableId)
    if (rawTable) {
      return {
        tableNo: rawTable.table_no,
        qrCodeUrl: normalizeQRCodeUrl(rawTable.qr_code_url)
      }
    }

    const viewTable = ensureArray(this.data.tables).find((item) => item.id === tableId)
    return {
      tableNo: viewTable?.table_no || '',
      qrCodeUrl: normalizeQRCodeUrl(viewTable?.qr_code_url)
    }
  },

  syncTableQRCodeUrl(tableId: number, qrCodeUrl: string) {
    const nextUrl = normalizeQRCodeUrl(qrCodeUrl)
    if (!nextUrl) return

    this.setData({
      rawTables: ensureArray(this.data.rawTables).map((item) => (
        item.id === tableId ? { ...item, qr_code_url: nextUrl } : item
      )),
      tables: ensureArray(this.data.tables).map((item) => (
        item.id === tableId ? { ...item, qr_code_url: nextUrl } : item
      ))
    })
  },

  async fetchTableQRCode(tableId: number, fallbackUrl = '') {
    try {
      const res = await tableManagementService.getTableQRCode(tableId)
      const qrCodeUrl = normalizeQRCodeUrl(res?.qr_code_url)
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
        wx.showToast({ title: '二维码刷新失败，已展示当前版本', icon: 'none' })
        return
      }

      this.setData({
        qrCodeLoading: false,
        qrCodeError: true,
        qrCodeErrorMessage: getErrorMessage(err, '生成二维码失败，请稍后重试'),
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
      this.fetchTableQRCode(tableId, fallbackUrl)
    }
  },

  onShowQRCode(e: WechatMiniprogram.TouchEvent) {
    const { id, no, url } = e.currentTarget.dataset as { id?: number, no?: string, url?: string }
    this.openTableQRCode({ id, tableNo: no, url, fetchFresh: !url })
  },

  onShowEditingQRCode() {
    if (!this.data.isEdit || !this.data.editingTableId) {
      wx.showToast({ title: '请先保存桌台后查看二维码', icon: 'none' })
      return
    }

    const context = this.getTableQRCodeContext(this.data.editingTableId)
    this.openTableQRCode({
      id: this.data.editingTableId,
      tableNo: context.tableNo,
      url: context.qrCodeUrl,
      fetchFresh: !context.qrCodeUrl
    })
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
    if (!this.data.qrCodeTableId) return

    this.openTableQRCode({
      id: this.data.qrCodeTableId,
      tableNo: this.data.qrCodeTableNo,
      url: this.data.qrCodeImageUrl,
      fetchFresh: true
    })
  },

  async onDownloadQRCode() {
    if (this.data.qrCodeDownloading || !this.data.qrCodeImageUrl) return

    this.setData({ qrCodeDownloading: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const downloadResult = await new Promise<WechatMiniprogram.DownloadFileSuccessCallbackResult>((resolve, reject) => {
        wx.downloadFile({
          url: this.data.qrCodeImageUrl,
          success: (res) => {
            if (res.statusCode >= 200 && res.statusCode < 300 && res.tempFilePath) {
              resolve(res)
              return
            }

            reject(new Error('download failed'))
          },
          fail: reject
        })
      })

      await new Promise<void>((resolve, reject) => {
        wx.saveImageToPhotosAlbum({
          filePath: downloadResult.tempFilePath,
          success: () => resolve(),
          fail: reject
        })
      })

      wx.showToast({ title: '二维码已保存到相册', icon: 'success' })
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
  },

  onAddTable() {
    const formData = createDefaultFormData()
    this.setData({
      formVisible: true,
      isEdit: false,
      editingTableId: 0,
      tableImages: [],
      uploadFiles: [],
      pendingMediaIds: [],
      pendingImagePreviews: [],
      formData,
      formStatusSelection: buildTableStatusSelection(formData.status)
    })
  },

  onTableClick(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const table = this.data.tables.find((item) => item.id === id)
    if (!table) return

    const normalizedStatus = getTableStatusDisplay(table.status).normalizedStatus as TableStatus

    this.setData({
      formVisible: true,
      isEdit: true,
      editingTableId: id,
      uploadFiles: [],
      pendingMediaIds: [],
      pendingImagePreviews: [],
      formData: {
        table_no: table.table_no,
        table_type: (table.table_type === 'room' ? 'room' : 'table'),
        capacity: table.capacity,
        description: table.description || '',
        minimum_spend_yuan: typeof table.minimum_spend === 'number' ? (table.minimum_spend / 100).toFixed(2) : '',
        status: normalizedStatus,
        access_code: '',
        tag_ids: Array.isArray(table.tags) ? table.tags.map((tag) => tag.id) : []
      },
      formStatusSelection: buildTableStatusSelection(normalizedStatus)
    })

    this.loadTableImages(id)
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  async loadTableImages(tableId: number) {
    try {
      const res = await tableManagementService.getTableImages(tableId)
      const normalizedImages = toSafeTableImages(res?.images)
      this.setData({ tableImages: normalizedImages })
    } catch (err) {
      logger.error('Load table images failed', err)
      this.setData({ tableImages: [] })
    }
  },

  async onImageAdd(e: WechatMiniprogram.CustomEvent<{ files?: Array<{ url?: string }> }>) {
    if (this.data.imageUploading || this.data.imageMutating) return

    const selectedFiles = Array.isArray(e.detail?.files)
      ? e.detail.files.filter((file): file is { url: string } => typeof file?.url === 'string' && !!file.url)
      : []
    if (!selectedFiles.length) {
      return
    }

    let uploadFiles = [...this.data.uploadFiles]
    uploadFiles.push(...selectedFiles.map((file) => ({
      url: file.url,
      localPath: file.url,
      status: 'loading' as const
    })))
    this.setData({ imageUploading: true, uploadFiles })

    try {
      wx.showLoading({ title: '添加图片中...' })

      for (const file of selectedFiles) {
        const pendingIndex = findUploadFileIndex(uploadFiles, file.url)
        if (pendingIndex < 0) {
          continue
        }

        try {
          const uploaded = await tableManagementService.uploadTableImageFile(file.url)
          const { mediaId, displayUrl } = uploaded
          if (!mediaId) {
            throw new Error('missing media id')
          }

          const previewUrl = displayUrl || file.url
          uploadFiles = replaceUploadFileAt(uploadFiles, pendingIndex, {
            url: previewUrl,
            localPath: file.url,
            mediaId,
            status: this.data.isEdit ? 'loading' : 'done'
          })
          this.setData({ uploadFiles })

          if (this.data.isEdit && this.data.editingTableId > 0) {
            await tableManagementService.uploadTableImage(this.data.editingTableId, { media_asset_id: mediaId })
            await this.loadTableImages(this.data.editingTableId)
            uploadFiles = removeUploadFileAt(uploadFiles, pendingIndex)
            this.setData({ uploadFiles })
          } else {
            const pendingMediaIds = ensureArray(this.data.pendingMediaIds)
            this.setData({
              pendingMediaIds: [...pendingMediaIds, mediaId],
              pendingImagePreviews: uploadFiles.map((item) => item.url)
            })
          }
        } catch (uploadErr) {
          logger.error('Choose/upload table image failed', uploadErr)
          uploadFiles = replaceUploadFileAt(uploadFiles, pendingIndex, {
            url: file.url,
            localPath: file.url,
            status: 'failed'
          })
          this.setData({ uploadFiles })
          wx.showToast({ title: getErrorMessage(uploadErr, '图片上传失败，请稍后重试'), icon: 'none' })
        }
      }
    } catch (err) {
      logger.error('Choose/upload table image failed', err)
      wx.showToast({ title: getErrorMessage(err, '图片上传失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ imageUploading: false })
    }
  },

  onUploadFileRemove(e: WechatMiniprogram.CustomEvent<{ index?: number }>) {
    const index = Number(e.detail?.index)
    if (!Number.isInteger(index) || index < 0) {
      return
    }

    const uploadFiles = removeUploadFileAt(this.data.uploadFiles, index)
    this.setData({ uploadFiles })

    if (!this.data.isEdit) {
      const nextMediaIds = [...ensureArray(this.data.pendingMediaIds)]
      nextMediaIds.splice(index, 1)
      this.setData({
        pendingMediaIds: nextMediaIds,
        pendingImagePreviews: uploadFiles.map((item) => item.url)
      })
    }
  },

  async onDeleteImage(e: WechatMiniprogram.TouchEvent) {
    const { imageId, index } = e.currentTarget.dataset as { imageId?: number, index?: number }

    if (this.data.imageMutating || this.data.imageUploading) return

    if (this.data.isEdit && this.data.editingTableId > 0 && imageId) {
      this.setData({ imageMutating: true, imageMutatingImageId: imageId })
      wx.showLoading({ title: '删除图片中...' })
      try {
        await tableManagementService.deleteTableImage(this.data.editingTableId, imageId)
        await this.loadTableImages(this.data.editingTableId)
      } catch (err) {
        logger.error('Delete table image failed', err)
        wx.showToast({ title: getErrorMessage(err, '删除图片失败，请稍后重试'), icon: 'none' })
      } finally {
        wx.hideLoading()
        this.setData({ imageMutating: false, imageMutatingImageId: 0 })
      }
      return
    }

    if (!this.data.isEdit && typeof index === 'number') {
      return
    }
  },

  async onSetPrimaryImage(e: WechatMiniprogram.TouchEvent) {
    const { imageId } = e.currentTarget.dataset as { imageId?: number }
    if (!this.data.isEdit || !this.data.editingTableId || !imageId || this.data.imageMutating || this.data.imageUploading) return

    this.setData({ imageMutating: true, imageMutatingImageId: imageId })
    wx.showLoading({ title: '设置主图中...' })
    try {
      await tableManagementService.setPrimaryTableImage(this.data.editingTableId, imageId)
      await this.loadTableImages(this.data.editingTableId)
    } catch (err) {
      logger.error('Set primary table image failed', err)
      wx.showToast({ title: getErrorMessage(err, '设置主图失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ imageMutating: false, imageMutatingImageId: 0 })
    }
  },

  resetFormState() {
    const formData = createDefaultFormData()
    this.setData({
      formVisible: false,
      isEdit: false,
      editingTableId: 0,
      tableImages: [],
      uploadFiles: [],
      pendingMediaIds: [],
      pendingImagePreviews: [],
      formData,
      formStatusSelection: buildTableStatusSelection(formData.status)
    })
  },

  async uploadPendingImages(tableId: number) {
    const pendingMediaIds = ensureArray(this.data.pendingMediaIds)
    if (!pendingMediaIds.length) {
      return { failedCount: 0 }
    }

    let failedCount = 0
    for (const mediaId of pendingMediaIds) {
      try {
        await tableManagementService.uploadTableImage(tableId, { media_asset_id: mediaId })
      } catch (err) {
        failedCount += 1
        logger.error('Bind pending table image failed', err)
      }
    }

    return { failedCount }
  },

  onPreviewImage(e: WechatMiniprogram.TouchEvent) {
    const { url } = e.currentTarget.dataset as { url?: string }
    if (!url) return
    const finalUrl = getPublicImageUrl(url)
    if (!finalUrl) {
      wx.showToast({ title: '图片地址无效', icon: 'none' })
      return
    }
    wx.previewImage({ urls: [finalUrl], current: finalUrl })
  },

  onTextInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) return
    const value = e.detail?.value || ''
    this.setData({ [`formData.${field}`]: value })
  },

  onCapacityInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const value = Number(e.detail?.value || 0)
    this.setData({ 'formData.capacity': Number.isFinite(value) ? value : 0 })
  },

  onTypeChange(e: WechatMiniprogram.CustomEvent<{ value: 'table' | 'room' }>) {
    const value = e.detail?.value === 'room' ? 'room' : 'table'
    this.setData({ 'formData.table_type': value })
  },

  onStatusChange(e: WechatMiniprogram.CustomEvent<{ value: TableStatus }>) {
    const value = e.detail?.value
    if (!value) return
    this.setData({
      'formData.status': value,
      formStatusSelection: buildTableStatusSelection(value)
    })
  },

  onTagChange(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const tagIds = values.map((value) => Number(value)).filter((id) => Number.isFinite(id) && id > 0)
    this.setData({ 'formData.tag_ids': tagIds })
  },

  async onCreateTag() {
    if (this.data.tagSubmitting) return

    wx.showModal({
      title: '新增标签',
      editable: true,
      placeholderText: '请输入标签名称',
      success: async (res) => {
        if (!res.confirm) return

        const name = (res.content || '').trim()
        if (!name) {
          wx.showToast({ title: '标签名称不能为空', icon: 'none' })
          return
        }

        this.setData({ tagSubmitting: true })
        try {
          const created = await TagService.createTag({ name, type: 'table' })
          const availableTags = ensureArray(this.data.availableTags)
          const selectedTagIds = ensureArray(this.data.formData.tag_ids)
          const nextTags = [...availableTags, { id: created.id, name: created.name }]
          const nextSelected = selectedTagIds.includes(created.id)
            ? selectedTagIds
            : [...selectedTagIds, created.id]

          this.setData({
            availableTags: nextTags,
            'formData.tag_ids': nextSelected
          })
        } catch (err) {
          logger.error('Create table tag failed', err)
          wx.showToast({ title: '新增标签失败', icon: 'none' })
        } finally {
          this.setData({ tagSubmitting: false })
        }
      }
    })
  },

  onGenerateQRCode() {
    if (!this.data.isEdit || !this.data.editingTableId) {
      wx.showToast({ title: '请先保存桌台后生成二维码', icon: 'none' })
      return
    }

    const context = this.getTableQRCodeContext(this.data.editingTableId)
    this.openTableQRCode({
      id: this.data.editingTableId,
      tableNo: context.tableNo,
      url: context.qrCodeUrl,
      fetchFresh: true
    })
  },

  async onSubmitForm() {
    if (this.data.formSubmitting) return

    const formData = this.data.formData
    const tableNo = (formData.table_no || '').trim()
    const accessCode = (formData.access_code || '').trim()
    if (!tableNo) {
      wx.showToast({ title: '请填写桌号', icon: 'none' })
      return
    }

    if (!Number.isInteger(formData.capacity) || formData.capacity < 1 || formData.capacity > 100) {
      wx.showToast({ title: '人数需在1-100之间', icon: 'none' })
      return
    }

    if (accessCode && (accessCode.length < 4 || accessCode.length > 32)) {
      wx.showToast({ title: '访问码需为4-32位', icon: 'none' })
      return
    }

    let minimumSpend: number | undefined
    if (formData.minimum_spend_yuan && formData.minimum_spend_yuan.trim()) {
      const parsed = Number(formData.minimum_spend_yuan)
      if (!Number.isFinite(parsed) || parsed < 0) {
        wx.showToast({ title: '最低消费金额不合法', icon: 'none' })
        return
      }
      minimumSpend = Math.round(parsed * 100)
    }

    const createPayload: CreateTableRequest = {
      table_no: tableNo,
      table_type: formData.table_type,
      capacity: formData.capacity,
      description: (formData.description || '').trim() || undefined,
      minimum_spend: minimumSpend,
      access_code: accessCode || undefined,
      tag_ids: formData.tag_ids
    }

    this.setData({ formSubmitting: true })
    try {
      if (this.data.isEdit && this.data.editingTableId > 0) {
        const updatePayload: UpdateTableRequest = {
          ...createPayload,
          status: formData.status
        }
        await tableManagementService.updateTable(this.data.editingTableId, updatePayload)
        this.resetFormState()
        await this.loadTables()
      } else {
        const created = await tableManagementService.createTable(createPayload)

        const { failedCount } = await this.uploadPendingImages(created.id)

        this.resetFormState()
        await this.loadTables()

        if (failedCount > 0) {
          wx.showToast({ title: '桌台已创建，部分图片添加失败，请进入编辑页重试', icon: 'none', duration: 3000 })
        }
      }
    } catch (err) {
      logger.error('Submit table form failed', err)
      wx.showToast({ title: getErrorMessage(err, '保存失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ formSubmitting: false })
    }
  }
})
