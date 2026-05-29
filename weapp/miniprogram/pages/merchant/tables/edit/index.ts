import { tableManagementService, type CreateTableRequest, type TableImageResponse, type UpdateTableRequest } from '../../../../api/table-device-management'
import { type TableResponse } from '../../_main_shared/api/table'
import { TagService } from '../../_main_shared/api/dish'
import { getStableBarHeights } from '../../../../utils/responsive'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { settleAll, isSettledFulfilled } from '../../../../utils/promise'
import {
  createDefaultTableFormData,
  ensureArray,
  isPermissionDeniedError,
  isUserCancelledError,
  normalizeQRCodeUrl,
  saveTableQRCodePosterToAlbum,
  TABLE_UPLOAD_FILE_STATUS,
  toSafeTableImages,
  toSafeTagOptions,
  toSafeUploadFiles,
  type TableFormData,
  type TableTagOption,
  type TableUploadFile
} from '../../_utils/merchant-tables-shared'
import {
  buildPersistedUploadFile,
  buildSelectedTagState,
  buildTableQRCodeContext,
  buildTableSubmitPayload,
  findUploadFileIndex,
  FormInputDetail,
  hasTableTagName,
  mapTableDetailToFormData,
  mergeSelectableTableTags,
  pickPendingBoundFiles,
  removeUploadFileAt,
  removeWarningMessageSegment,
  replaceUploadFileAt,
  splitTableImageFiles,
  TableEditPageOptions,
  TableImageRole,
  validateTableBeforeSubmit
} from '../shared/table-edit-view'

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
    qrCodeVisible: false,
    qrCodeLoading: false,
    qrCodeDownloading: false,
    qrCodeError: false,
    qrCodeErrorMessage: '',
    qrCodeImageUrl: '',
    qrCodeTableNo: '',
    availableTags: [] as TableTagOption[],
    selectedTagState: {} as Record<string, boolean>,
    createTagDialogVisible: false,
    createTagInputValue: '',
    tagSubmitting: false,
    coverUploadFiles: [] as TableUploadFile[],
    galleryUploadFiles: [] as TableUploadFile[],
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
      const { coverFiles, galleryFiles } = splitTableImageFiles(tableImages)

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
        coverUploadFiles: coverFiles,
        galleryUploadFiles: galleryFiles,
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
    if (this.data.tagSubmitting) return
    this.setData({
      createTagDialogVisible: true,
      createTagInputValue: ''
    })
  },

  onCloseCreateTagDialog() {
    if (this.data.tagSubmitting) return
    this.setData({ createTagDialogVisible: false, createTagInputValue: '' })
  },

  onCreateTagInputChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    this.setData({ createTagInputValue: (e.detail?.value || '').replace(/^\s+/, '') })
  },

  async onConfirmCreateTagDialog() {
    if (this.data.tagSubmitting) {
      return
    }

    const name = this.data.createTagInputValue.trim()
    if (!name) {
      wx.showToast({ title: '标签名称不能为空', icon: 'none' })
      return
    }

    if (hasTableTagName(this.data.availableTags, name)) {
      wx.showToast({ title: '标签名称已存在', icon: 'none' })
      return
    }

    this.setData({ tagSubmitting: true })

    try {
      const created = await TagService.createTag({ name, type: 'table' })
      const availableTags = this.data.availableTags.some((tag) => tag.id === created.id)
        ? this.data.availableTags
        : [...this.data.availableTags, { id: created.id, name: created.name }]
      const nextTagIds = this.data.formData.tag_ids.includes(created.id)
        ? this.data.formData.tag_ids
        : [...this.data.formData.tag_ids, created.id]

      this.setData({
        createTagDialogVisible: false,
        createTagInputValue: '',
        availableTags,
        selectedTagState: buildSelectedTagState(nextTagIds),
        loadWarningMessage: removeWarningMessageSegment(this.data.loadWarningMessage, '桌台标签暂未同步，仍可继续编辑基础信息'),
        'formData.tag_ids': nextTagIds
      })
    } catch (err) {
      logger.error('Create table tag failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '创建标签失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ tagSubmitting: false })
    }
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
    if (!value || value === this.data.formData.table_type) return
    this.setData({ 'formData.table_type': value })
  },

  onStatusSelect(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: TableFormData['status'] }
    if (!value || value === this.data.formData.status) return
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

  getRoleUploadFiles(role: TableImageRole): TableUploadFile[] {
    return toSafeUploadFiles(role === 'cover' ? this.data.coverUploadFiles : this.data.galleryUploadFiles)
  },

  setRoleUploadFiles(role: TableImageRole, files: TableUploadFile[]) {
    this.setData(role === 'cover' ? { coverUploadFiles: files } : { galleryUploadFiles: files })
  },

  onCoverAdd(e: WechatMiniprogram.CustomEvent<{ files?: Array<{ url?: string }> }>) {
    void this.handleImageAdd('cover', e)
  },

  onGalleryAdd(e: WechatMiniprogram.CustomEvent<{ files?: Array<{ url?: string }> }>) {
    void this.handleImageAdd('gallery', e)
  },

  async handleImageAdd(role: TableImageRole, e: WechatMiniprogram.CustomEvent<{ files?: Array<{ url?: string }> }>) {
    if (this.data.imageUploading || this.data.imageMutating) {
      return
    }

    const selectedFiles = Array.isArray(e.detail?.files)
      ? e.detail.files.filter((file): file is { url: string } => typeof file?.url === 'string' && !!file.url)
      : []
    if (!selectedFiles.length) {
      return
    }

    let uploadFiles = this.getRoleUploadFiles(role)
    const pendingFiles = selectedFiles.slice(0, role === 'cover' ? 1 : selectedFiles.length).map((file) => ({
      url: file.url,
      localPath: file.url,
      status: TABLE_UPLOAD_FILE_STATUS.loading
    }))

    uploadFiles = role === 'cover'
      ? pendingFiles
      : [...uploadFiles, ...pendingFiles]

    this.setData({
      imageUploading: true,
      ...(role === 'cover'
        ? { coverUploadFiles: uploadFiles }
        : { galleryUploadFiles: uploadFiles })
    })

    try {
      wx.showLoading({ title: '上传图片中...' })

      for (const file of pendingFiles) {
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
            status: TABLE_UPLOAD_FILE_STATUS.done
          })
          this.setRoleUploadFiles(role, uploadFiles)

          if (this.data.isEdit && this.data.tableId > 0) {
            const savedImage = await tableManagementService.uploadTableImage(this.data.tableId, {
              media_asset_id: uploaded.mediaId,
              is_primary: role === 'cover' ? true : undefined
            })

            const persistedFile = buildPersistedUploadFile(savedImage, uploaded.displayUrl || file.url, uploaded.mediaId)
            if (role === 'cover') {
              const currentGalleryFiles = this.getRoleUploadFiles('gallery')
              const demotedCoverFiles = this.getRoleUploadFiles('cover').filter((coverFile, coverIndex) => coverIndex !== pendingIndex && !!coverFile.imageId)

              this.setData({
                coverUploadFiles: [persistedFile],
                galleryUploadFiles: [...currentGalleryFiles, ...demotedCoverFiles]
              })
            } else {
              uploadFiles = replaceUploadFileAt(uploadFiles, pendingIndex, persistedFile)
              this.setRoleUploadFiles(role, uploadFiles)
            }
          }
        } catch (err) {
          logger.error('Upload table image failed', err)
          uploadFiles = replaceUploadFileAt(uploadFiles, pendingIndex, {
            url: file.url,
            localPath: file.url,
            status: TABLE_UPLOAD_FILE_STATUS.failed
          })
          this.setRoleUploadFiles(role, uploadFiles)
          wx.showToast({ title: getErrorUserMessage(err, '图片上传失败，请稍后重试'), icon: 'none' })
        }
      }
    } finally {
      wx.hideLoading()
      this.setData({ imageUploading: false })
    }
  },

  onCoverRemove(e: WechatMiniprogram.CustomEvent<{ index?: number }>) {
    void this.handleUploadFileRemove('cover', e)
  },

  onGalleryRemove(e: WechatMiniprogram.CustomEvent<{ index?: number }>) {
    void this.handleUploadFileRemove('gallery', e)
  },

  async handleUploadFileRemove(role: TableImageRole, e: WechatMiniprogram.CustomEvent<{ index?: number }>) {
    const index = Number(e.detail?.index)
    if (!Number.isInteger(index) || index < 0) {
      return
    }

    const uploadFiles = this.getRoleUploadFiles(role)
    const targetFile = uploadFiles[index]
    if (!targetFile) {
      return
    }

    if (this.data.isEdit && this.data.tableId > 0 && typeof targetFile.imageId === 'number' && targetFile.imageId > 0) {
      const nextFiles = removeUploadFileAt(uploadFiles, index)
      this.setRoleUploadFiles(role, nextFiles)
      this.setData({ imageMutating: true })
      wx.showLoading({ title: '删除图片中...' })

      try {
        await tableManagementService.deleteTableImage(this.data.tableId, targetFile.imageId)
      } catch (err) {
        logger.error('Delete table image failed', err)
        this.setRoleUploadFiles(role, uploadFiles)
        wx.showToast({ title: getErrorUserMessage(err, '删除图片失败，请稍后重试'), icon: 'none' })
      } finally {
        wx.hideLoading()
        this.setData({ imageMutating: false })
      }
      return
    }

    this.setRoleUploadFiles(role, removeUploadFileAt(uploadFiles, index))
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

    const context = buildTableQRCodeContext(this.data.formData, this.data.qrCodeTableNo, this.data.qrCodeImageUrl)
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
  },

  async uploadPendingImages(tableId: number) {
    let failedCount = 0

    for (const file of pickPendingBoundFiles(this.getRoleUploadFiles('cover')).slice(0, 1)) {
      try {
        await tableManagementService.uploadTableImage(tableId, {
          media_asset_id: Number(file.mediaId),
          is_primary: true
        })
      } catch (err) {
        failedCount += 1
        logger.error('Bind pending table cover failed', err)
      }
    }

    for (const file of pickPendingBoundFiles(this.getRoleUploadFiles('gallery'))) {
      try {
        await tableManagementService.uploadTableImage(tableId, { media_asset_id: Number(file.mediaId) })
      } catch (err) {
        failedCount += 1
        logger.error('Bind pending table gallery image failed', err)
      }
    }

    return { failedCount }
  },

  async notifyPreviousPage() {
    const pages = getCurrentPages()
    const prevPage = pages[pages.length - 2] as { refreshAll?: (showLoading?: boolean) => Promise<void> | void } | undefined
    if (prevPage?.refreshAll) await prevPage.refreshAll(false)
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading) {
      return
    }

    const validationMessage = validateTableBeforeSubmit(this.data.formData, [
      ...this.getRoleUploadFiles('cover'),
      ...this.getRoleUploadFiles('gallery')
    ])
    if (validationMessage) {
      wx.showToast({ title: validationMessage, icon: 'none' })
      return
    }

    this.setData({ submitting: true })

    try {
      const payload = buildTableSubmitPayload(this.data.formData)

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

  await this.notifyPreviousPage()
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit table failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '保存失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }

})
