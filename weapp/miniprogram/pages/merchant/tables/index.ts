import { getStableBarHeights } from '../../../utils/responsive'
import { tableManagementService, CreateTableRequest, TableImageResponse, TableResponse, UpdateTableRequest } from '../../../api/table-device-management'
import { TagService } from '../../../api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

type TableTab = 'all' | 'table' | 'room'

interface TableView extends TableResponse {
  statusLabel: string
  statusTheme: string
}

interface TableTagOption {
  id: number
  name: string
}

interface TableFormData {
  table_no: string
  table_type: 'table' | 'room'
  capacity: number
  description: string
  minimum_spend_yuan: string
  status: 'available' | 'occupied' | 'disabled'
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

function normalizeQRCodeUrl(path?: string): string {
  if (!path) return ''
  return getPublicImageUrl(path)
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

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    initialError: false,
    initialErrorMessage: '',
    listLoaded: false,
    formSubmitting: false,
    currentTab: 'all' as TableTab,
    tables: [] as TableView[],
    rawTables: [] as TableResponse[],
    availableTags: [] as TableTagOption[],
    tableImages: [] as TableImageResponse[],
    pendingMediaIds: [] as number[],
    pendingImagePreviews: [] as string[],
    imageUploading: false,
    imageMutating: false,
    imageMutatingImageId: 0,
    tagSubmitting: false,
    formVisible: false,
    isEdit: false,
    editingTableId: 0,
    formData: createDefaultFormData()
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.normalizeListState()
    this.loadTables()
    this.loadAvailableTags()
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

  async loadTables() {
    if (this.data.loading) return
    this.setData({
      loading: true,
      initialError: false,
      initialErrorMessage: ''
    })
    
    try {
      // API 支持 table_type 过滤，但这里我们先拉取全部在前端切也行，或者根据 tab 传参
      const type = this.data.currentTab === 'all' ? undefined : this.data.currentTab
      const res = await tableManagementService.listTables(type)
      
      const rawTables = Array.isArray(res?.tables)
        ? res.tables.filter((table): table is TableResponse => !!table && typeof table === 'object')
        : []

      const formatted = rawTables.map((t) => this.formatTable(t))
      this.setData({ 
        tables: formatted,
        rawTables,
        listLoaded: true,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load tables failed', err)
      const message = getErrorMessage(err, '加载桌台失败，请稍后重试')
      if (!this.data.listLoaded) {
        this.setData({
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    this.loadTables()
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
          this.loadTables()
        } catch (err) {
          logger.error('Release table failed', err)
          wx.showToast({ title: '操作失败', icon: 'none' })
        }
      }
    })
  },

  onShowQRCode(e: WechatMiniprogram.TouchEvent) {
    const { id, url } = e.currentTarget.dataset as { id?: number, url?: string }
    this.previewTableQRCode(id, url)
  },

  previewTableQRCode(id?: number, url?: string) {
    if (id) {
      tableManagementService.getTableQRCode(id)
        .then((res) => {
          const qrCodeUrl = normalizeQRCodeUrl(res?.qr_code_url)
          if (!qrCodeUrl) {
            wx.showToast({ title: '暂无二维码', icon: 'none' })
            return
          }
          wx.previewImage({ urls: [qrCodeUrl], current: qrCodeUrl })
        })
        .catch((err) => {
          logger.error('Get table qrcode failed', err)

          const fallbackUrl = normalizeQRCodeUrl(url)
          if (fallbackUrl) {
            wx.previewImage({ urls: [fallbackUrl], current: fallbackUrl })
            return
          }

          wx.showToast({ title: '获取二维码失败', icon: 'none' })
        })
      return
    }

    if (!url) {
      return wx.showToast({ title: '暂无二维码', icon: 'none' })
    }

    const finalUrl = normalizeQRCodeUrl(url)
    if (!finalUrl) {
      return wx.showToast({ title: '暂无二维码', icon: 'none' })
    }
    wx.previewImage({
      urls: [finalUrl],
      current: finalUrl
    })
  },

  onAddTable() {
    this.setData({
      formVisible: true,
      isEdit: false,
      editingTableId: 0,
      tableImages: [],
      pendingMediaIds: [],
      pendingImagePreviews: [],
      formData: createDefaultFormData()
    })
  },

  onTableClick(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const table = this.data.tables.find((item) => item.id === id)
    if (!table) return

    this.setData({
      formVisible: true,
      isEdit: true,
      editingTableId: id,
      pendingMediaIds: [],
      pendingImagePreviews: [],
      formData: {
        table_no: table.table_no,
        table_type: (table.table_type === 'room' ? 'room' : 'table'),
        capacity: table.capacity,
        description: table.description || '',
        minimum_spend_yuan: typeof table.minimum_spend === 'number' ? (table.minimum_spend / 100).toFixed(2) : '',
        status: table.status === 'occupied' ? 'occupied' : table.status === 'disabled' ? 'disabled' : 'available',
        access_code: '',
        tag_ids: Array.isArray(table.tags) ? table.tags.map((tag) => tag.id) : []
      }
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

  async onChooseImage() {
    if (this.data.imageUploading || this.data.imageMutating) return

    try {
      const chooseRes = await wx.chooseMedia({
        count: 1,
        mediaType: ['image'],
        sourceType: ['album', 'camera']
      })

      const filePath = chooseRes.tempFiles?.[0]?.tempFilePath
      if (!filePath) return

      this.setData({ imageUploading: true })
      wx.showLoading({ title: '添加图片中...' })

      const uploaded = await tableManagementService.uploadTableImageFile(filePath)
      const { mediaId, displayUrl } = uploaded
      if (!mediaId) {
        wx.showToast({ title: '上传失败', icon: 'none' })
        return
      }

      if (this.data.isEdit && this.data.editingTableId > 0) {
        await tableManagementService.uploadTableImage(this.data.editingTableId, { media_asset_id: mediaId })
        await this.loadTableImages(this.data.editingTableId)
      } else {
        const pendingMediaIds = ensureArray(this.data.pendingMediaIds)
        const pendingImagePreviews = ensureArray(this.data.pendingImagePreviews)
        this.setData({
          pendingMediaIds: [...pendingMediaIds, mediaId],
          pendingImagePreviews: [...pendingImagePreviews, displayUrl]
        })
      }
    } catch (err) {
      logger.error('Choose/upload table image failed', err)
      wx.showToast({ title: getErrorMessage(err, '图片上传失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ imageUploading: false })
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
      const next = [...ensureArray(this.data.pendingMediaIds)]
      const nextPreviews = [...ensureArray(this.data.pendingImagePreviews)]
      next.splice(index, 1)
      nextPreviews.splice(index, 1)
      this.setData({ pendingMediaIds: next, pendingImagePreviews: nextPreviews })
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
    this.setData({
      formVisible: false,
      isEdit: false,
      editingTableId: 0,
      tableImages: [],
      pendingMediaIds: [],
      pendingImagePreviews: [],
      formData: createDefaultFormData()
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

  onStatusChange(e: WechatMiniprogram.CustomEvent<{ value: 'available' | 'occupied' | 'disabled' }>) {
    const value = e.detail?.value
    if (!value) return
    this.setData({ 'formData.status': value })
  },

  onTagChange(e: WechatMiniprogram.CustomEvent<{ value: string[] }>) {
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

  onDeleteTag(e: WechatMiniprogram.TouchEvent) {
    if (this.data.tagSubmitting) return
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id) return

    const selectedTagIds = ensureArray(this.data.formData.tag_ids)
    if (!selectedTagIds.includes(id)) return

    const isEdit = this.data.isEdit && this.data.editingTableId > 0
    const modalContent = isEdit
      ? `确认取消标签「${name || ''}」与当前桌台的关联吗？`
      : `确认取消标签「${name || ''}」的选择吗？`

    wx.showModal({
      title: isEdit ? '取消关联标签' : '取消选择标签',
      content: modalContent,
      success: async (res) => {
        if (!res.confirm) return

        this.setData({ tagSubmitting: true })
        try {
          if (isEdit) {
            await tableManagementService.deleteTableTag(this.data.editingTableId, id)
          }

          this.setData({
            'formData.tag_ids': selectedTagIds.filter((tagId) => tagId !== id)
          })
        } catch (err) {
          logger.error('Remove table tag failed', err)
          wx.showToast({ title: isEdit ? '取消关联失败' : '取消选择失败', icon: 'none' })
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
    this.previewTableQRCode(this.data.editingTableId)
  },

  async onSubmitForm() {
    if (this.data.formSubmitting) return

    const formData = this.data.formData
    const tableNo = (formData.table_no || '').trim()
    if (!tableNo) {
      wx.showToast({ title: '请填写桌号', icon: 'none' })
      return
    }

    if (!Number.isInteger(formData.capacity) || formData.capacity < 1 || formData.capacity > 100) {
      wx.showToast({ title: '人数需在1-100之间', icon: 'none' })
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
      access_code: (formData.access_code || '').trim() || undefined,
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
