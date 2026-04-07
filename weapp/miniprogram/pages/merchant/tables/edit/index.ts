import { tableManagementService, type CreateTableRequest, type TableImageResponse, type UpdateTableRequest } from '../../../../api/table-device-management'
import { type TableResponse } from '../../../../api/table'
import { TagService } from '../../../../api/dish'
import { getStableBarHeights } from '../../../../utils/responsive'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { settleAll, isSettledFulfilled } from '../../../../utils/promise'
import {
  createDefaultTableFormData,
  downloadRemoteImageToAlbum,
  ensureArray,
  isPermissionDeniedError,
  isUserCancelledError,
  normalizeQRCodeUrl,
  normalizeTableBusinessStatus,
  TABLE_UPLOAD_FILE_STATUS,
  toSafeTableImages,
  toSafeTagOptions,
  toSafeUploadFiles,
  type TableFormData,
  type TableTagOption,
  type TableUploadFile
} from '../shared'

interface TableEditPageOptions {
  id?: string
}

interface TableQRCodeContext {
  tableNo: string
  qrCodeUrl: string
}

function buildSelectedTagState(tagIds: number[]): Record<string, boolean> {
  return tagIds.reduce<Record<string, boolean>>((result, id) => {
    result[String(id)] = true
    return result
  }, {})
}

function mergeSelectableTableTags(primaryTags: TableTagOption[], fallbackTags: TableTagOption[]): TableTagOption[] {
  const mergedTags: TableTagOption[] = []
  const seenTagIds = new Set<number>()

  for (const tag of [...primaryTags, ...fallbackTags]) {
    if (!tag || !Number.isFinite(tag.id) || tag.id <= 0) {
      continue
    }
    if (seenTagIds.has(tag.id)) {
      continue
    }
    seenTagIds.add(tag.id)
    mergedTags.push(tag)
  }

  return mergedTags
}

function mapTableDetailToFormData(table: TableResponse): TableFormData {
  const normalizedStatus = normalizeTableBusinessStatus(table.status)

  return {
    table_no: table.table_no || '',
    table_type: table.table_type === 'room' ? 'room' : 'table',
    capacity: typeof table.capacity === 'number' ? table.capacity : 4,
    description: table.description || '',
    minimum_spend_yuan: typeof table.minimum_spend === 'number' && table.minimum_spend > 0
      ? (table.minimum_spend / 100).toFixed(2)
      : '',
    status: normalizedStatus,
    tag_ids: ensureArray(table.tags)
      .map((tag) => Number(tag.id))
      .filter((id) => Number.isFinite(id) && id > 0)
  }
}

function findUploadFileIndex(files: TableUploadFile[], localPath: string): number {
  return files.findIndex((file) => file.localPath === localPath)
}

function replaceUploadFileAt(files: TableUploadFile[], index: number, file: TableUploadFile): TableUploadFile[] {
  if (index < 0 || index >= files.length) {
    return files
  }

  const nextFiles = [...files]
  nextFiles[index] = file
  return nextFiles
}

function removeUploadFileAt(files: TableUploadFile[], index: number): TableUploadFile[] {
  if (index < 0 || index >= files.length) {
    return files
  }

  const nextFiles = [...files]
  nextFiles.splice(index, 1)
  return nextFiles
}

Page({
  data: {
    navBarHeight: 88,
    isEdit: false,
    tableId: 0,
    bootstrapped: false,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loadWarningMessage: '',
    submitting: false,
    imageUploading: false,
    imageMutating: false,
    imageMutatingImageId: 0,
    qrCodeVisible: false,
    qrCodeLoading: false,
    qrCodeDownloading: false,
    qrCodeError: false,
    qrCodeErrorMessage: '',
    qrCodeImageUrl: '',
    qrCodeTableNo: '',
    availableTags: [] as TableTagOption[],
    selectedTagState: {} as Record<string, boolean>,
    tableImages: [] as TableImageResponse[],
    uploadFiles: [] as TableUploadFile[],
    formData: createDefaultTableFormData(),
    statusOptions: [
      { label: '空闲', value: 'available' },
      { label: '占用中', value: 'occupied' },
      { label: '已预订', value: 'reserved' },
      { label: '停用', value: 'disabled' }
    ]
  },

  onLoad(options: TableEditPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    const tableId = options.id ? Number(options.id) : 0

    this.setData({
      navBarHeight,
      isEdit: tableId > 0,
      tableId
    })

    void this.loadPageData()
  },

  onShow() {
    if (!this.data.bootstrapped || this.data.initialLoading || this.data.submitting) {
      return
    }

    void this.refreshTagsSilently()
  },

  async loadPageData() {
    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      loadWarningMessage: ''
    })

    try {
      const results = await settleAll([
        TagService.listTags('table'),
        this.data.isEdit
          ? tableManagementService.getTableDetail(this.data.tableId)
          : Promise.resolve(null as TableResponse | null),
        this.data.isEdit
          ? tableManagementService.getTableImages(this.data.tableId)
          : Promise.resolve({ images: [] as TableImageResponse[] })
      ] as const)

      const [tagsResult, detailResult, imagesResult] = results
      if (this.data.isEdit && !isSettledFulfilled(detailResult)) {
        throw detailResult.reason
      }

      const detail = this.data.isEdit && isSettledFulfilled(detailResult) ? detailResult.value : null
      const detailTags = toSafeTagOptions(detail?.tags)
      const availableTags = mergeSelectableTableTags(
        isSettledFulfilled(tagsResult) ? toSafeTagOptions(tagsResult.value) : [],
        detailTags
      )
      const formData = detail ? mapTableDetailToFormData(detail) : createDefaultTableFormData()
      const tableImages = isSettledFulfilled(imagesResult)
        ? toSafeTableImages(imagesResult.value?.images)
        : []

      const warningMessages: string[] = []
      if (!isSettledFulfilled(tagsResult)) {
        warningMessages.push('桌台标签暂未同步，仍可继续编辑基础信息')
      }
      if (this.data.isEdit && !isSettledFulfilled(imagesResult)) {
        warningMessages.push('桌台图片列表暂未同步，请稍后重试')
      }

      this.setData({
        bootstrapped: true,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        loadWarningMessage: warningMessages.filter(Boolean).join('；'),
        availableTags,
        selectedTagState: buildSelectedTagState(formData.tag_ids),
        tableImages,
        uploadFiles: [],
        formData,
        qrCodeImageUrl: detail ? normalizeQRCodeUrl(detail.qr_code_url) : '',
        qrCodeTableNo: detail?.table_no || ''
      })
    } catch (err) {
      logger.error('Load table edit page failed', err)
      this.setData({
        bootstrapped: false,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '桌台编辑页加载失败，请重试')
      })
    }
  },

  async refreshTagsSilently() {
    try {
      const tags = await TagService.listTags('table')
      const availableTags = mergeSelectableTableTags(
        toSafeTagOptions(tags),
        ensureArray(this.data.availableTags).filter((tag) => this.data.formData.tag_ids.includes(tag.id))
      )

      this.setData({ availableTags })
    } catch (err) {
      logger.warn('Refresh table tags silently failed', err)
    }
  },

  onRetry() {
    void this.loadPageData()
  },

  onManageTags() {
    wx.navigateTo({ url: '/pages/merchant/tables/tags/index' })
  },

  onInputChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof TableFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: e.detail?.value || '' })
  },

  onCapacityChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const value = Number(e.detail?.value || 0)
    this.setData({
      'formData.capacity': Number.isFinite(value) ? value : 0
    })
  },

  onTypeSelect(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: 'table' | 'room' }
    if (!value || value === this.data.formData.table_type) {
      return
    }

    this.setData({ 'formData.table_type': value })
  },

  onStatusSelect(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: TableFormData['status'] }
    if (!value || value === this.data.formData.status) {
      return
    }

    this.setData({ 'formData.status': value })
  },

  onTagToggle(e: WechatMiniprogram.TouchEvent) {
    const { tagId } = e.currentTarget.dataset as { tagId?: number }
    if (!tagId) {
      return
    }

    const selectedTagIds = ensureArray(this.data.formData.tag_ids)
    const nextTagIds = selectedTagIds.includes(tagId)
      ? selectedTagIds.filter((id) => id !== tagId)
      : [...selectedTagIds, tagId]

    this.setData({
      'formData.tag_ids': nextTagIds,
      selectedTagState: buildSelectedTagState(nextTagIds)
    })
  },

  async onImageAdd(e: WechatMiniprogram.CustomEvent<{ files?: Array<{ url?: string }> }>) {
    if (this.data.imageUploading || this.data.imageMutating) {
      return
    }

    const selectedFiles = Array.isArray(e.detail?.files)
      ? e.detail.files.filter((file): file is { url: string } => typeof file?.url === 'string' && !!file.url)
      : []
    if (!selectedFiles.length) {
      return
    }

    let uploadFiles = [...toSafeUploadFiles(this.data.uploadFiles)]
    uploadFiles.push(...selectedFiles.map((file) => ({
      url: file.url,
      localPath: file.url,
      status: TABLE_UPLOAD_FILE_STATUS.loading
    })))

    this.setData({
      imageUploading: true,
      uploadFiles
    })

    try {
      wx.showLoading({ title: '上传图片中...' })

      for (const file of selectedFiles) {
        const pendingIndex = findUploadFileIndex(uploadFiles, file.url)
        if (pendingIndex < 0) {
          continue
        }

        try {
          const uploaded = await tableManagementService.uploadTableImageFile(file.url)
          if (!uploaded.mediaId) {
            throw new Error('missing media id')
          }

          uploadFiles = replaceUploadFileAt(uploadFiles, pendingIndex, {
            url: uploaded.displayUrl || file.url,
            localPath: file.url,
            mediaId: uploaded.mediaId,
            status: this.data.isEdit ? TABLE_UPLOAD_FILE_STATUS.loading : TABLE_UPLOAD_FILE_STATUS.done
          })
          this.setData({ uploadFiles })

          if (this.data.isEdit && this.data.tableId > 0) {
            await tableManagementService.uploadTableImage(this.data.tableId, { media_asset_id: uploaded.mediaId })
            await this.loadTableImages(this.data.tableId)

            uploadFiles = removeUploadFileAt(uploadFiles, pendingIndex)
            this.setData({ uploadFiles })
          }
        } catch (err) {
          logger.error('Upload table image failed', err)
          uploadFiles = replaceUploadFileAt(uploadFiles, pendingIndex, {
            url: file.url,
            localPath: file.url,
            status: TABLE_UPLOAD_FILE_STATUS.failed
          })
          this.setData({ uploadFiles })
          wx.showToast({ title: getErrorUserMessage(err, '图片上传失败，请稍后重试'), icon: 'none' })
        }
      }
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

    this.setData({
      uploadFiles: removeUploadFileAt(toSafeUploadFiles(this.data.uploadFiles), index)
    })
  },

  async loadTableImages(tableId: number) {
    try {
      const response = await tableManagementService.getTableImages(tableId)
      this.setData({ tableImages: toSafeTableImages(response?.images) })
    } catch (err) {
      logger.error('Load table images failed', err)
      this.setData({ tableImages: [] })
    }
  },

  async onSetPrimaryImage(e: WechatMiniprogram.TouchEvent) {
    const { imageId } = e.currentTarget.dataset as { imageId?: number }
    if (!this.data.isEdit || !this.data.tableId || !imageId || this.data.imageMutating || this.data.imageUploading) {
      return
    }

    this.setData({ imageMutating: true, imageMutatingImageId: imageId })
    wx.showLoading({ title: '设置主图中...' })

    try {
      await tableManagementService.setPrimaryTableImage(this.data.tableId, imageId)
      await this.loadTableImages(this.data.tableId)
    } catch (err) {
      logger.error('Set primary table image failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '设置主图失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ imageMutating: false, imageMutatingImageId: 0 })
    }
  },

  async onDeleteImage(e: WechatMiniprogram.TouchEvent) {
    const { imageId } = e.currentTarget.dataset as { imageId?: number }
    if (!this.data.isEdit || !this.data.tableId || !imageId || this.data.imageMutating || this.data.imageUploading) {
      return
    }

    this.setData({ imageMutating: true, imageMutatingImageId: imageId })
    wx.showLoading({ title: '删除图片中...' })

    try {
      await tableManagementService.deleteTableImage(this.data.tableId, imageId)
      await this.loadTableImages(this.data.tableId)
    } catch (err) {
      logger.error('Delete table image failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '删除图片失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ imageMutating: false, imageMutatingImageId: 0 })
    }
  },

  onPreviewImage(e: WechatMiniprogram.TouchEvent) {
    const { url } = e.currentTarget.dataset as { url?: string }
    if (!url) {
      return
    }

    wx.previewImage({ urls: [url], current: url })
  },

  getTableQRCodeContext(): TableQRCodeContext {
    return {
      tableNo: this.data.formData.table_no || this.data.qrCodeTableNo || '',
      qrCodeUrl: normalizeQRCodeUrl(this.data.qrCodeImageUrl)
    }
  },

  async fetchTableQRCode(fallbackUrl = '') {
    if (!this.data.tableId) {
      return
    }

    try {
      const response = await tableManagementService.getTableQRCode(this.data.tableId)
      const qrCodeUrl = normalizeQRCodeUrl(response?.qr_code_url)
      if (!qrCodeUrl) {
        throw new Error('missing qr code url')
      }

      this.setData({
        qrCodeLoading: false,
        qrCodeError: false,
        qrCodeErrorMessage: '',
        qrCodeImageUrl: qrCodeUrl,
        qrCodeTableNo: this.data.formData.table_no || this.data.qrCodeTableNo
      })
    } catch (err) {
      logger.error('Fetch table qrcode failed', err)

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
        qrCodeErrorMessage: getErrorUserMessage(err, '生成二维码失败，请稍后重试'),
        qrCodeImageUrl: ''
      })
    }
  },

  openTableQRCode(fetchFresh = false) {
    if (!this.data.isEdit || !this.data.tableId) {
      wx.showToast({ title: '请先保存桌台后查看二维码', icon: 'none' })
      return
    }

    const context = this.getTableQRCodeContext()
    const shouldFetch = fetchFresh || !context.qrCodeUrl

    this.setData({
      qrCodeVisible: true,
      qrCodeLoading: shouldFetch,
      qrCodeDownloading: false,
      qrCodeError: false,
      qrCodeErrorMessage: '',
      qrCodeImageUrl: context.qrCodeUrl,
      qrCodeTableNo: context.tableNo
    })

    if (shouldFetch) {
      void this.fetchTableQRCode(context.qrCodeUrl)
    }
  },

  onShowQRCode() {
    this.openTableQRCode(false)
  },

  onGenerateQRCode() {
    this.openTableQRCode(true)
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
    this.openTableQRCode(true)
  },

  async onDownloadQRCode() {
    if (this.data.qrCodeDownloading || !this.data.qrCodeImageUrl) {
      return
    }

    this.setData({ qrCodeDownloading: true })
    wx.showLoading({ title: '保存中...' })

    try {
      await downloadRemoteImageToAlbum(this.data.qrCodeImageUrl)
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

  validateBeforeSubmit(): string {
    const formData = this.data.formData
    const tableNo = (formData.table_no || '').trim()
    const uploadFiles = toSafeUploadFiles(this.data.uploadFiles)

    if (!tableNo) {
      return '请填写桌号或包间名'
    }

    if (!Number.isInteger(formData.capacity) || formData.capacity < 1 || formData.capacity > 100) {
      return '人数需在 1 到 100 之间'
    }

    if (uploadFiles.some((file) => file.status === TABLE_UPLOAD_FILE_STATUS.loading)) {
      return '图片仍在上传中，请稍候'
    }

    if (uploadFiles.some((file) => file.status === TABLE_UPLOAD_FILE_STATUS.failed)) {
      return '有图片上传失败，请删除后重试'
    }

    if (formData.minimum_spend_yuan && formData.minimum_spend_yuan.trim()) {
      const parsed = Number(formData.minimum_spend_yuan)
      if (!Number.isFinite(parsed) || parsed < 0) {
        return '最低消费金额不合法'
      }
    }

    return ''
  },

  buildSubmitPayload() {
    const formData = this.data.formData
    const minimumSpend = formData.minimum_spend_yuan && formData.minimum_spend_yuan.trim()
      ? Math.round(Number(formData.minimum_spend_yuan) * 100)
      : undefined

    return {
      table_no: (formData.table_no || '').trim(),
      table_type: formData.table_type,
      capacity: formData.capacity,
      description: (formData.description || '').trim() || undefined,
      minimum_spend: minimumSpend,
      tag_ids: ensureArray(formData.tag_ids)
    }
  },

  async uploadPendingImages(tableId: number) {
    const pendingFiles = toSafeUploadFiles(this.data.uploadFiles).filter((file) => typeof file.mediaId === 'number' && file.mediaId > 0)
    let failedCount = 0

    for (const file of pendingFiles) {
      try {
        await tableManagementService.uploadTableImage(tableId, { media_asset_id: Number(file.mediaId) })
      } catch (err) {
        failedCount += 1
        logger.error('Bind pending table image failed', err)
      }
    }

    return { failedCount }
  },

  notifyPreviousPage() {
    const pages = getCurrentPages()
    const prevPage = pages[pages.length - 2] as { refreshAll?: (showLoading?: boolean) => Promise<void> | void } | undefined
    if (prevPage?.refreshAll) {
      void prevPage.refreshAll(false)
    }
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading) {
      return
    }

    const validationMessage = this.validateBeforeSubmit()
    if (validationMessage) {
      wx.showToast({ title: validationMessage, icon: 'none' })
      return
    }

    this.setData({ submitting: true })

    try {
      const payload = this.buildSubmitPayload()

      if (this.data.isEdit && this.data.tableId > 0) {
        const updatePayload: UpdateTableRequest = {
          ...payload,
          status: this.data.formData.status
        }
        await tableManagementService.updateTable(this.data.tableId, updatePayload)
      } else {
        const createPayload: CreateTableRequest = payload
        const created = await tableManagementService.createTable(createPayload)
        const { failedCount } = await this.uploadPendingImages(created.id)
        if (failedCount > 0) {
          wx.showToast({ title: '桌台已创建，部分图片关联失败，请稍后进入编辑页重试', icon: 'none', duration: 3000 })
        }
      }

      this.notifyPreviousPage()
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit table failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '保存失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }

})
