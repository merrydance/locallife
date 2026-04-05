import { getStableBarHeights } from '../../../../utils/responsive'
import {
  DishManagementService,
  DishResponse,
  CreateDishRequest,
  UpdateDishRequest,
  TagService,
  TagInfo,
  CustomizationGroup,
  CustomizationGroupInput
} from '../../../../api/dish'
import { waitForPublicMediaDisplayUrl } from '../../../../api/onboarding'
import { getPublicImageUrl } from '../../../../utils/image'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface UploadFileItem {
  url: string
  status?: 'loading' | 'done' | 'failed'
  remotePath?: string
}

interface DishEditPageOptions {
  id?: string
}

interface FormInputDetail {
  value: string
}

interface CategoryOption {
  label: string
  value: string
}

interface CustomizationOptionDraft {
  key: string
  name: string
  extraPriceYuan: string
}

interface CustomizationGroupDraft {
  key: string
  name: string
  is_required: boolean
  options: CustomizationOptionDraft[]
}

const MAX_CUSTOMIZATION_GROUPS = 20
const FEATURED_TAG_NAMES = new Set(['推荐', '热卖'])

function buildDraftKey(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}

function createEmptyOptionDraft(): CustomizationOptionDraft {
  return {
    key: buildDraftKey('option'),
    name: '',
    extraPriceYuan: ''
  }
}

function createEmptyGroupDraft(): CustomizationGroupDraft {
  return {
    key: buildDraftKey('group'),
    name: '',
    is_required: false,
    options: []
  }
}

function mapCustomizationGroupsToDrafts(groups?: CustomizationGroup[]): CustomizationGroupDraft[] {
  if (!Array.isArray(groups) || groups.length === 0) {
    return []
  }

  return groups.map((group) => ({
    key: `group_${group.id}`,
    name: group.name || '',
    is_required: !!group.is_required,
    options: Array.isArray(group.options) && group.options.length > 0
      ? group.options.map((option) => ({
        key: `option_${option.id}`,
        name: option.tag_name || '',
        extraPriceYuan: option.extra_price ? (option.extra_price / 100).toFixed(2) : ''
      }))
      : []
  }))
}

function toDishPreviewUrl(imageUrl?: string | null): string {
  return imageUrl ? getPublicImageUrl(imageUrl) : ''
}

function buildDishFileList(previewUrl: string): UploadFileItem[] {
  return previewUrl ? [{ url: previewUrl, status: 'done' }] : []
}

Page({
  data: {
    navBarHeight: 88,
    isEdit: false,
    dishId: 0,
    loading: false,
    submitting: false,
    imageUploading: false,
    persistedImageAssetId: 0,
    persistedImagePreviewUrl: '',
    isIPhoneX: false,
    isFeatured: false,  // 推荐
    isHotSelling: false, // 热卖
    tagSubmitting: false,
    availableDishTags: [] as TagInfo[],
    selectedDishTagIds: [] as number[],
    formData: {
      name: '',
      description: '',
      category_id: 0,
      price: 0, // 分
      member_price: 0, // 分
      is_online: true,
      is_available: true,
      sort_order: 0,
      prepare_time: 15,
      image_asset_id: 0,   // 图片媒体资产 ID（新）
      image_preview_url: '' // 本地/CDN 预览 URL
    },
    displayPrice: '', // 元
    displayMemberPrice: '', // 元
    selectedCategoryName: '',
    selectedCategoryValue: '',
    categoryVisible: false,
    categoryOptions: [] as CategoryOption[],
    fileList: [] as UploadFileItem[],
    customizationGroups: [] as CustomizationGroupDraft[]
  },

  onLoad(options: DishEditPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    const deviceInfo = wx.getDeviceInfo ? wx.getDeviceInfo() : wx.getSystemInfoSync()
    const model = deviceInfo?.model || ''
    const isIPhoneX = model.includes('iPhone X') || model.includes('iPhone 11') || model.includes('iPhone 12') || model.includes('iPhone 13')
    
    this.setData({ 
      navBarHeight, 
      isIPhoneX,
      isEdit: !!options.id,
      dishId: options.id ? Number(options.id) : 0
    })

    this.loadCategories()
    this.loadDishTags()
    if (this.data.isEdit) {
      this.loadDishDetail()
    }
  },

  async loadDishTags() {
    try {
      const tags = await TagService.listDishTags()
      this.setData({
        availableDishTags: (Array.isArray(tags) ? tags : []).filter((tag) => !FEATURED_TAG_NAMES.has(tag.name))
      })
    } catch (err) {
      logger.error('Load dish tags failed', err)
      this.setData({ availableDishTags: [] })
    }
  },

  async loadCategories() {
    try {
      const list = await DishManagementService.getDishCategories()
      const categoryOptions = list.map((c) => ({ label: c.name, value: String(c.id) }))
      const updates: WechatMiniprogram.Page.DataOption = { categoryOptions }

      if (this.data.isEdit && this.data.formData.category_id > 0) {
        const hit = categoryOptions.find((item) => Number(item.value) === this.data.formData.category_id)
        if (hit) {
          updates.selectedCategoryValue = hit.value
          updates.selectedCategoryName = hit.label
        }
      }

      if (!this.data.isEdit && this.data.formData.category_id <= 0 && categoryOptions.length > 0) {
        updates['formData.category_id'] = Number(categoryOptions[0].value)
        updates.selectedCategoryValue = categoryOptions[0].value
        updates.selectedCategoryName = categoryOptions[0].label
      }
      this.setData(updates)
    } catch (err) {
      logger.error('Load categories failed', err)
    }
  },

  async loadDishDetail() {
    this.setData({ loading: true })
    try {
      const res = await DishManagementService.getDishDetail(this.data.dishId)
      const previewUrl = toDishPreviewUrl(res.image_url)
      let customizationGroups = mapCustomizationGroupsToDrafts(res.customization_groups)

      try {
        const customizationResponse = await DishManagementService.getDishCustomizations(this.data.dishId)
        customizationGroups = mapCustomizationGroupsToDrafts(customizationResponse)
      } catch (error) {
        logger.error('Load dish customizations failed', error)
      }

      this.setData({
        formData: {
          name: res.name,
          description: res.description,
          category_id: res.category_id || 0,
          price: res.price,
          member_price: res.member_price || 0,
          is_online: res.is_online,
          is_available: res.is_available,
          sort_order: res.sort_order || 0,
          prepare_time: res.prepare_time || 15,
          image_asset_id: res.image_asset_id || 0,
          image_preview_url: previewUrl
        },
        persistedImageAssetId: res.image_asset_id || 0,
        persistedImagePreviewUrl: previewUrl,
        displayPrice: (res.price / 100).toFixed(2),
        displayMemberPrice: res.member_price ? (res.member_price / 100).toFixed(2) : '',
        fileList: buildDishFileList(previewUrl),
        selectedCategoryName: res.category_name || '',
        selectedCategoryValue: res.category_id ? String(res.category_id) : '',
        isFeatured: res.tags?.some((t: { name: string }) => t.name === '推荐') ?? false,
        isHotSelling: res.tags?.some((t: { name: string }) => t.name === '热卖') ?? false,
        selectedDishTagIds: (res.tags || [])
          .filter((tag) => !FEATURED_TAG_NAMES.has(tag.name))
          .map((tag) => tag.id)
          .filter((id) => Number.isFinite(id) && id > 0),
        customizationGroups
      })
    } catch (err) {
      logger.error('Load dish detail failed', err)
      wx.showToast({ title: '加载菜品失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  // ==================== 输入处理 ====================

  onInputChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) return
    const { value } = e.detail
    if (field === 'prepare_time') {
      const prepareTime = Number.parseInt(value, 10)
      this.setData({ [`formData.${field}`]: Number.isFinite(prepareTime) ? prepareTime : 0 })
      return
    }
    if (field === 'sort_order') {
      const sortOrder = Number.parseInt(value, 10)
      this.setData({ [`formData.${field}`]: Number.isFinite(sortOrder) ? sortOrder : 0 })
      return
    }
    this.setData({ [`formData.${field}`]: value.replace(/^\s+/, '') })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) return
    const { value } = e.detail
    this.setData({ [`formData.${field}`]: value })
  },

  onFeaturedTagToggle(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { tag } = e.currentTarget.dataset as { tag?: string }
    if (!tag) return
    const { value } = e.detail
    if (tag === '推荐') this.setData({ isFeatured: value })
    else if (tag === '热卖') this.setData({ isHotSelling: value })
  },

  onDishTagChange(e: WechatMiniprogram.CustomEvent<{ value: string[] }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedDishTagIds = values
      .map((value) => Number(value))
      .filter((id) => Number.isFinite(id) && id > 0)

    this.setData({ selectedDishTagIds })
  },

  onCreateDishTag() {
    if (this.data.tagSubmitting) {
      return
    }

    wx.showModal({
      title: '新增菜品标签',
      editable: true,
      placeholderText: '请输入标签名称',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        const name = (res.content || '').trim()
        if (!name) {
          wx.showToast({ title: '标签名称不能为空', icon: 'none' })
          return
        }
        if (FEATURED_TAG_NAMES.has(name)) {
          wx.showToast({ title: '推荐和热卖请使用下方排序标签开关', icon: 'none' })
          return
        }

        this.setData({ tagSubmitting: true })
        try {
          const created = await TagService.createTag({ name, type: 'dish' })
          const availableDishTags = this.data.availableDishTags.some((tag) => tag.id === created.id)
            ? this.data.availableDishTags
            : [...this.data.availableDishTags, created]
          const selectedDishTagIds = this.data.selectedDishTagIds.includes(created.id)
            ? this.data.selectedDishTagIds
            : [...this.data.selectedDishTagIds, created.id]

          this.setData({
            availableDishTags,
            selectedDishTagIds
          })
        } catch (err) {
          logger.error('Create dish tag failed', err)
          wx.showToast({ title: '新增标签失败，请重试', icon: 'none' })
        } finally {
          this.setData({ tagSubmitting: false })
        }
      }
    })
  },

  onPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const val = e.detail.value.trim()
    const parsed = Number.parseFloat(val)
    this.setData({
      displayPrice: val,
      'formData.price': Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 0
    })
  },

  onMemberPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const val = e.detail.value.trim()
    const parsed = Number.parseFloat(val)
    this.setData({
      displayMemberPrice: val,
      'formData.member_price': Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 0
    })
  },

  // ==================== 图片处理 ====================

  buildPreviewUrl(path: string): string {
    return getPublicImageUrl(path)
  },

  applyPersistedDishState(dish: DishResponse, fallbackPreviewUrl = '') {
    const previewUrl = toDishPreviewUrl(dish.image_url) || fallbackPreviewUrl
    const imageAssetID = dish.image_asset_id || 0

    this.setData({
      dishId: dish.id || this.data.dishId,
      isEdit: true,
      persistedImageAssetId: imageAssetID,
      persistedImagePreviewUrl: previewUrl,
      'formData.image_asset_id': imageAssetID,
      'formData.image_preview_url': previewUrl,
      'formData.category_id': dish.category_id || this.data.formData.category_id,
      fileList: buildDishFileList(previewUrl),
      selectedCategoryName: dish.category_name || this.data.selectedCategoryName,
      selectedCategoryValue: dish.category_id ? String(dish.category_id) : this.data.selectedCategoryValue
    })
  },

  async finalizePendingDishImage(mediaId: number, fallbackPreviewUrl: string) {
    try {
      const remoteUrl = await waitForPublicMediaDisplayUrl(mediaId)
      if (!remoteUrl || this.data.formData.image_asset_id !== mediaId) {
        return
      }

      this.setData({
        fileList: buildDishFileList(remoteUrl),
        'formData.image_preview_url': remoteUrl
      })
    } catch (err) {
      logger.warn('Finalize dish image preview failed', err)
      if (!this.data.fileList.length && fallbackPreviewUrl && this.data.formData.image_asset_id === mediaId) {
        this.setData({
          fileList: buildDishFileList(fallbackPreviewUrl),
          'formData.image_preview_url': fallbackPreviewUrl
        })
      }
    }
  },

  async onImageAdd(e: WechatMiniprogram.CustomEvent<{ files: Array<{ url: string }> }>) {
    const files = Array.isArray(e.detail?.files) ? e.detail.files : []
    const localPath = files[0]?.url
    if (!localPath) {
      wx.showToast({ title: '请选择有效图片', icon: 'none' })
      return
    }

    this.setData({
      imageUploading: true,
      fileList: [{ url: localPath, status: 'loading' }]
    })

    try {
      const { mediaId, displayUrl } = await DishManagementService.uploadDishImage(localPath)
      const previewUrl = displayUrl || localPath

      this.setData({
        fileList: buildDishFileList(previewUrl),
        'formData.image_asset_id': mediaId,
        'formData.image_preview_url': previewUrl
      })

      if (!displayUrl) {
        void this.finalizePendingDishImage(mediaId, localPath)
      }
    } catch (err) {
      logger.error('Upload image failed', err)
      this.setData({ fileList: [] })
      wx.showToast({ title: '上传失败', icon: 'none' })
    } finally {
      this.setData({ imageUploading: false })
    }
  },

  onImagePreview(e: WechatMiniprogram.CustomEvent<{ index?: number }>) {
    const urls = this.data.fileList
      .map((item) => item.url)
      .filter((url) => !!url)

    if (!urls.length) return

    const index = Number(e.detail?.index || 0)
    wx.previewImage({
      current: urls[index] || urls[0],
      urls
    })
  },

  onImageRemove() {
    if (this.data.isEdit && this.data.persistedImageAssetId > 0) {
      const persistedPreviewUrl = this.data.persistedImagePreviewUrl
      this.setData({
        fileList: buildDishFileList(persistedPreviewUrl),
        'formData.image_asset_id': this.data.persistedImageAssetId,
        'formData.image_preview_url': persistedPreviewUrl
      })
      wx.showToast({ title: '当前暂不支持删除已发布图片，可重新上传替换', icon: 'none' })
      return
    }

    this.setData({
      fileList: [],
      'formData.image_asset_id': 0,
      'formData.image_preview_url': ''
    })
  },

  // ==================== 分类选择 ====================

  showCategoryPicker() {
    if (!this.data.categoryOptions.length) {
      wx.showToast({ title: '暂无分类，请先创建分类', icon: 'none' })
      return
    }

    this.setData({ categoryVisible: true })
  },

  onCategoryConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null, label: string[] | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const labels = Array.isArray(e.detail?.label) ? e.detail.label : []
    const selectedValue = String(values[0] ?? this.data.selectedCategoryValue ?? '')
    const val = Number(selectedValue || this.data.formData.category_id)
    const label = String(labels[0] ?? this.data.selectedCategoryName ?? '')

    if (!Number.isFinite(val) || val <= 0) {
      this.setData({ categoryVisible: false })
      wx.showToast({ title: '请选择分类', icon: 'none' })
      return
    }

    this.setData({
      'formData.category_id': val,
      selectedCategoryValue: selectedValue,
      selectedCategoryName: label,
      categoryVisible: false
    })
  },

  onCategoryCancel() {
    this.setData({ categoryVisible: false })
  },

  // ==================== 规格编辑 ====================

  onAddCustomizationGroup() {
    if (this.data.customizationGroups.length >= MAX_CUSTOMIZATION_GROUPS) {
      wx.showToast({ title: `最多添加${MAX_CUSTOMIZATION_GROUPS}组规格`, icon: 'none' })
      return
    }

    this.setData({
      customizationGroups: [...this.data.customizationGroups, createEmptyGroupDraft()]
    })
  },

  onRemoveCustomizationGroup(e: WechatMiniprogram.TouchEvent) {
    const { groupIndex } = e.currentTarget.dataset as { groupIndex?: number }
    if (typeof groupIndex !== 'number') return

    const targetGroup = this.data.customizationGroups[groupIndex]
    const groupName = targetGroup?.name?.trim()

    wx.showModal({
      title: '删除规格组',
      content: groupName
        ? `删除“${groupName}”后，组内规格项会一起移除。`
        : '删除后，组内规格项会一起移除。',
      success: (res) => {
        if (!res.confirm) {
          return
        }

        const customizationGroups = [...this.data.customizationGroups]
        customizationGroups.splice(groupIndex, 1)
        this.setData({ customizationGroups })
      }
    })
  },

  onCustomizationGroupNameInput(
    e: WechatMiniprogram.CustomEvent<FormInputDetail> & { currentTarget: { dataset: { groupIndex?: number } } }
  ) {
    const { groupIndex } = e.currentTarget.dataset
    if (typeof groupIndex !== 'number') return

    const customizationGroups = [...this.data.customizationGroups]
    customizationGroups[groupIndex] = {
      ...customizationGroups[groupIndex],
      name: e.detail.value.replace(/^\s+/, '')
    }
    this.setData({ customizationGroups })
  },

  onCustomizationGroupRequiredChange(
    e: WechatMiniprogram.CustomEvent<{ value: boolean }> & { currentTarget: { dataset: { groupIndex?: number } } }
  ) {
    const { groupIndex } = e.currentTarget.dataset
    if (typeof groupIndex !== 'number') return

    const customizationGroups = [...this.data.customizationGroups]
    customizationGroups[groupIndex] = {
      ...customizationGroups[groupIndex],
      is_required: !!e.detail.value
    }
    this.setData({ customizationGroups })
  },

  onAddCustomizationOption(e: WechatMiniprogram.TouchEvent) {
    const { groupIndex } = e.currentTarget.dataset as { groupIndex?: number }
    if (typeof groupIndex !== 'number') return

    const customizationGroups = [...this.data.customizationGroups]
    const group = customizationGroups[groupIndex]
    customizationGroups[groupIndex] = {
      ...group,
      options: [...group.options, createEmptyOptionDraft()]
    }
    this.setData({ customizationGroups })
  },

  onRemoveCustomizationOption(e: WechatMiniprogram.TouchEvent) {
    const { groupIndex, optionIndex } = e.currentTarget.dataset as { groupIndex?: number, optionIndex?: number }
    if (typeof groupIndex !== 'number' || typeof optionIndex !== 'number') return

    const customizationGroups = [...this.data.customizationGroups]
    const group = customizationGroups[groupIndex]
    const targetOption = group?.options?.[optionIndex]
    const optionName = targetOption?.name?.trim()
    const nextOptions = [...group.options]
    nextOptions.splice(optionIndex, 1)

    wx.showModal({
      title: '删除规格项',
      content: nextOptions.length === 0
        ? (optionName
          ? `删除“${optionName}”后，该规格组会变成空组，提交前需要重新添加规格项。`
          : '删除后，该规格组会变成空组，提交前需要重新添加规格项。')
        : (optionName ? `确认删除“${optionName}”吗？` : '确认删除当前规格项吗？'),
      success: (res) => {
        if (!res.confirm) {
          return
        }

        customizationGroups[groupIndex] = {
          ...group,
          options: nextOptions
        }
        this.setData({ customizationGroups })
      }
    })
  },

  onCustomizationOptionNameInput(
    e: WechatMiniprogram.CustomEvent<FormInputDetail> & { currentTarget: { dataset: { groupIndex?: number, optionIndex?: number } } }
  ) {
    const { groupIndex, optionIndex } = e.currentTarget.dataset
    if (typeof groupIndex !== 'number' || typeof optionIndex !== 'number') return

    const customizationGroups = [...this.data.customizationGroups]
    const group = customizationGroups[groupIndex]
    const options = [...group.options]
    options[optionIndex] = {
      ...options[optionIndex],
      name: e.detail.value.replace(/^\s+/, '')
    }
    customizationGroups[groupIndex] = { ...group, options }
    this.setData({ customizationGroups })
  },

  onCustomizationOptionPriceInput(
    e: WechatMiniprogram.CustomEvent<FormInputDetail> & { currentTarget: { dataset: { groupIndex?: number, optionIndex?: number } } }
  ) {
    const { groupIndex, optionIndex } = e.currentTarget.dataset
    if (typeof groupIndex !== 'number' || typeof optionIndex !== 'number') return

    const customizationGroups = [...this.data.customizationGroups]
    const group = customizationGroups[groupIndex]
    const options = [...group.options]
    options[optionIndex] = {
      ...options[optionIndex],
      extraPriceYuan: e.detail.value.trim()
    }
    customizationGroups[groupIndex] = { ...group, options }
    this.setData({ customizationGroups })
  },

  async buildCustomizationPayload(): Promise<CustomizationGroupInput[]> {
    const normalizedGroups = this.data.customizationGroups
      .map((group) => ({
        name: group.name.trim(),
        is_required: group.is_required,
        options: group.options
          .map((option) => ({
            name: option.name.trim(),
            extraPriceYuan: option.extraPriceYuan.trim()
          }))
          .filter((option) => option.name)
      }))
      .filter((group) => group.name || group.options.length > 0)

    if (!normalizedGroups.length) {
      return []
    }

    for (const group of normalizedGroups) {
      if (!group.name) {
        throw new Error('请填写规格组名称')
      }
      if (!group.options.length) {
        throw new Error(`规格组“${group.name}”至少需要一个规格项`)
      }

      for (const option of group.options) {
        if (option.extraPriceYuan) {
          const extraPrice = Number(option.extraPriceYuan)
          if (!Number.isFinite(extraPrice) || extraPrice < 0) {
            throw new Error(`规格选项「${option.name}」加价金额不合法`)
          }
        }
      }
    }

    const existingTags = await TagService.listCustomizationTags()
    const tagIdByName = new Map(existingTags.map((tag) => [tag.name, tag.id]))

    const optionNames = Array.from(new Set(
      normalizedGroups.reduce<string[]>((result, group) => {
        group.options.forEach((option) => {
          result.push(option.name)
        })
        return result
      }, [])
    ))

    for (const optionName of optionNames) {
      if (tagIdByName.has(optionName)) continue
      const created = await TagService.createTag({ name: optionName, type: 'customization' })
      tagIdByName.set(created.name, created.id)
    }

    return normalizedGroups.map((group, groupIndex) => ({
      name: group.name,
      is_required: group.is_required,
      sort_order: groupIndex,
      options: group.options.map((option, optionIndex) => ({
        tag_id: tagIdByName.get(option.name) || 0,
        extra_price: option.extraPriceYuan ? Math.round(Number(option.extraPriceYuan) * 100) : 0,
        sort_order: optionIndex
      }))
    }))
  },

  // ==================== 提交 ====================

  async ensureCategoryForSubmit(): Promise<number> {
    if (this.data.formData.category_id > 0) {
      return this.data.formData.category_id
    }

    if (this.data.categoryOptions.length > 0) {
      const first = this.data.categoryOptions[0]
      this.setData({
        'formData.category_id': Number(first.value),
        selectedCategoryValue: first.value,
        selectedCategoryName: first.label
      })
      return Number(first.value)
    }

    const created = await DishManagementService.createDishCategory({
      name: '默认分类',
      sort_order: 99
    })
    const createdOption: CategoryOption = { label: created.name, value: String(created.id) }
    this.setData({
      categoryOptions: [createdOption],
      'formData.category_id': created.id,
      selectedCategoryValue: String(created.id),
      selectedCategoryName: created.name
    })
    return created.id
  },

  buildSubmitPayload(categoryId: number): CreateDishRequest | UpdateDishRequest {
    const name = this.data.formData.name.trim()
    const description = this.data.formData.description.trim()
    const payload: CreateDishRequest | UpdateDishRequest = {
      name,
      category_id: categoryId,
      price: this.data.formData.price,
      is_online: this.data.formData.is_online,
      is_available: this.data.formData.is_available,
      prepare_time: this.data.formData.prepare_time,
      sort_order: this.data.formData.sort_order
    }

    if (description) {
      payload.description = description
    }
    if (this.data.formData.image_asset_id) {
      payload.image_asset_id = this.data.formData.image_asset_id
    }
    if (this.data.selectedDishTagIds.length > 0 || this.data.isEdit) {
      payload.tag_ids = this.data.selectedDishTagIds
    }

    // 编辑页采用整表单提交语义，会员价清空时也要显式下发 0，避免旧值残留在后端。
    if (this.data.isEdit || this.data.formData.member_price > 0) {
      payload.member_price = this.data.formData.member_price
    }

    return payload
  },

  buildFeaturedTags(): string[] {
    const featuredTags: string[] = []
    if (this.data.isFeatured) {
      featuredTags.push('推荐')
    }
    if (this.data.isHotSelling) {
      featuredTags.push('热卖')
    }
    return featuredTags
  },

  async onSubmit() {
    const { formData } = this.data
    if (!formData.name.trim()) {
      wx.showToast({ title: '请输入菜品名称', icon: 'none' })
      return
    }
    if (formData.price <= 0) {
      wx.showToast({ title: '请输入正确价格', icon: 'none' })
      return
    }
    if (formData.member_price > 0 && formData.member_price >= formData.price) {
      wx.showToast({ title: '会员价需小于售价', icon: 'none' })
      return
    }
    if (formData.prepare_time < 1 || formData.prepare_time > 120) {
      wx.showToast({ title: '出餐时间需在1-120分钟', icon: 'none' })
      return
    }
    if (formData.sort_order < 0 || formData.sort_order > 999) {
      wx.showToast({ title: '排序值需在0-999之间', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    let currentDishId = this.data.dishId
    let baseDishSaved = false
    try {
      const categoryId = await this.ensureCategoryForSubmit()
      const customizationGroups = await this.buildCustomizationPayload()
      const featuredTags = this.buildFeaturedTags()
      const payload = this.buildSubmitPayload(categoryId)

      if (this.data.isEdit) {
        const updatedDish = await DishManagementService.updateDish(this.data.dishId, payload as UpdateDishRequest)
        currentDishId = this.data.dishId
        baseDishSaved = true
        this.applyPersistedDishState(updatedDish, this.data.formData.image_preview_url)
      } else {
        const createdDish = await DishManagementService.createDish(payload as CreateDishRequest)
        currentDishId = createdDish.id
        baseDishSaved = true
        this.applyPersistedDishState(createdDish, this.data.formData.image_preview_url)
      }

      if (currentDishId > 0) {
        await DishManagementService.setDishFeaturedTags(currentDishId, featuredTags)
      }

      if (currentDishId > 0 && (this.data.isEdit || customizationGroups.length > 0)) {
        await DishManagementService.setDishCustomizations(currentDishId, { groups: customizationGroups })
      }

      const pages = getCurrentPages()
      const prevPage = pages[pages.length - 2] as { refreshAll?: () => void } | undefined
      if (prevPage?.refreshAll) {
        prevPage.refreshAll()
      }
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit dish failed', err)
      const message = baseDishSaved && currentDishId > 0
        ? '基础信息已保存，规格或标签同步失败，请重试'
        : getErrorUserMessage(err, '提交失败，请重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
