import { getStableBarHeights } from '../../../../utils/responsive'
import {
  DishManagementService,
  DishResponse,
  CreateDishRequest,
  UpdateDishRequest,
  TagService,
  TagInfo,
  CustomizationGroup,
  CustomizationGroupInput,
  DishCategory
} from '../../../../api/dish'
import { waitForPublicMediaDisplayUrl } from '../../../../api/onboarding'
import { getPublicImageUrl } from '../../../../utils/image'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { settleAll, isSettledFulfilled } from '../../../../utils/promise'

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

type CreatePopupMode = 'category' | 'tag' | ''
const FEATURED_TAG_NAMES = new Set(['推荐', '热卖'])

function mapCustomizationGroupsToDrafts(groups?: CustomizationGroup[] | null): CustomizationGroupDraft[] {
  if (!Array.isArray(groups) || groups.length === 0) {
    return []
  }

  return groups.map((group) => ({
    key: `group_${group.id}`,
    name: group.name || '',
    is_required: true,
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

function buildCategoryOptions(list: DishCategory[]): CategoryOption[] {
  return list.map((category) => ({
    label: category.name,
    value: String(category.id)
  }))
}

function resolveCategorySelection(
  categoryOptions: CategoryOption[],
  currentCategoryId: number,
  currentCategoryName: string,
  allowDefaultSelection: boolean
) {
  const matchedCategory = categoryOptions.find((item) => Number(item.value) === currentCategoryId)
  if (matchedCategory) {
    return {
      categoryId: Number(matchedCategory.value),
      categoryValue: matchedCategory.value,
      categoryName: matchedCategory.label
    }
  }

  if (allowDefaultSelection && currentCategoryId <= 0 && categoryOptions.length > 0) {
    return {
      categoryId: Number(categoryOptions[0].value),
      categoryValue: categoryOptions[0].value,
      categoryName: categoryOptions[0].label
    }
  }

  return {
    categoryId: currentCategoryId > 0 ? currentCategoryId : 0,
    categoryValue: currentCategoryId > 0 ? String(currentCategoryId) : '',
    categoryName: currentCategoryName || ''
  }
}

function mergeSelectableDishTags(primaryTags: TagInfo[], fallbackTags: TagInfo[]): TagInfo[] {
  const mergedTags: TagInfo[] = []
  const seenTagIds = new Set<number>()

  for (const tag of [...primaryTags, ...fallbackTags]) {
    if (!tag || !Number.isFinite(tag.id) || tag.id <= 0 || FEATURED_TAG_NAMES.has(tag.name)) {
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

function extractSelectedDishTagIds(tags?: TagInfo[] | null): number[] {
  return (Array.isArray(tags) ? tags : [])
    .filter((tag) => !FEATURED_TAG_NAMES.has(tag.name))
    .map((tag) => tag.id)
    .filter((id) => Number.isFinite(id) && id > 0)
}

function joinWarningMessages(messages: string[]): string {
  return messages.filter((message) => !!message).join('；')
}

function buildSelectedDishTagState(selectedDishTagIds: number[]): Record<string, boolean> {
  return selectedDishTagIds.reduce<Record<string, boolean>>((result, id) => {
    result[String(id)] = true
    return result
  }, {})
}

Page({
  data: {
    navBarHeight: 88,
    isEdit: false,
    dishId: 0,
    bootstrapped: false,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loadWarningMessage: '',
    submitting: false,
    imageUploading: false,
    persistedImageAssetId: 0,
    persistedImagePreviewUrl: '',
    isFeatured: false,
    isHotSelling: false,
    tagSubmitting: false,
    availableDishTags: [] as TagInfo[],
    selectedDishTagIds: [] as number[],
    selectedDishTagState: {} as Record<string, boolean>,
    formData: {
      name: '',
      description: '',
      category_id: 0,
      price: 0,
      member_price: 0,
      is_online: true,
      is_available: true,
      sort_order: 0,
      prepare_time: 15,
      image_asset_id: 0,
      image_preview_url: ''
    },
    displayPrice: '',
    displayMemberPrice: '',
    selectedCategoryName: '',
    selectedCategoryValue: '',
    categoryVisible: false,
    createPopupVisible: false,
    createPopupMode: '' as CreatePopupMode,
    createInputValue: '',
    categoryCreateSubmitting: false,
    categoryOptions: [] as CategoryOption[],
    fileList: [] as UploadFileItem[],
    customizationGroups: [] as CustomizationGroupDraft[]
  },

  onLoad(options: DishEditPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    const dishId = options.id ? Number(options.id) : 0

    this.setData({
      navBarHeight,
      isEdit: dishId > 0,
      dishId
    })

    void this.loadPageData()
  },

  onShow() {
    if (!this.data.bootstrapped || this.data.initialLoading || this.data.submitting) {
      return
    }

    void this.refreshCategoriesSilently()
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
        DishManagementService.getDishCategories(),
        TagService.listDishTags(),
        this.data.isEdit
          ? DishManagementService.getDishDetail(this.data.dishId)
          : Promise.resolve(null as DishResponse | null),
        this.data.isEdit
          ? DishManagementService.getDishCustomizations(this.data.dishId)
          : Promise.resolve(null as CustomizationGroup[] | null)
      ] as const)

      const [categoriesResult, tagsResult, detailResult, customizationResult] = results
      if (!isSettledFulfilled(categoriesResult)) {
        throw categoriesResult.reason
      }
      if (this.data.isEdit && !isSettledFulfilled(detailResult)) {
        throw detailResult.reason
      }

      const detail = this.data.isEdit && isSettledFulfilled(detailResult) ? detailResult.value : null
      const categoryOptions = buildCategoryOptions(categoriesResult.value || [])
      const categorySelection = resolveCategorySelection(
        categoryOptions,
        detail?.category_id || this.data.formData.category_id,
        detail?.category_name || this.data.selectedCategoryName,
        !this.data.isEdit
      )

      const warningMessages: string[] = []
      if (!isSettledFulfilled(tagsResult)) {
        warningMessages.push('普通标签未同步，仍可继续编辑基础信息')
      }
      if (this.data.isEdit && !isSettledFulfilled(customizationResult)) {
        warningMessages.push('规格明细使用详情回退数据，提交前请再检查一遍')
      }

      const detailTags = Array.isArray(detail?.tags) ? detail.tags : []
      const availableDishTags = mergeSelectableDishTags(
        isSettledFulfilled(tagsResult) ? tagsResult.value : [],
        detailTags
      )
      const selectedDishTagIds = extractSelectedDishTagIds(detailTags)
      const previewUrl = toDishPreviewUrl(detail?.image_url)
      const customizationGroups = this.data.isEdit
        ? mapCustomizationGroupsToDrafts(
          isSettledFulfilled(customizationResult) ? customizationResult.value : detail?.customization_groups
        )
        : []

      this.setData({
        bootstrapped: true,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        loadWarningMessage: joinWarningMessages(warningMessages),
        availableDishTags,
        selectedDishTagIds,
        selectedDishTagState: buildSelectedDishTagState(selectedDishTagIds),
        categoryOptions,
        selectedCategoryName: categorySelection.categoryName,
        selectedCategoryValue: categorySelection.categoryValue,
        persistedImageAssetId: detail?.image_asset_id || 0,
        persistedImagePreviewUrl: previewUrl,
        displayPrice: detail ? (detail.price / 100).toFixed(2) : '',
        displayMemberPrice: detail?.member_price ? (detail.member_price / 100).toFixed(2) : '',
        fileList: buildDishFileList(previewUrl),
        isFeatured: detailTags.some((tag) => tag.name === '推荐'),
        isHotSelling: detailTags.some((tag) => tag.name === '热卖'),
        customizationGroups,
        formData: {
          name: detail?.name || '',
          description: detail?.description || '',
          category_id: categorySelection.categoryId,
          price: detail?.price || 0,
          member_price: detail?.member_price || 0,
          is_online: detail?.is_online ?? true,
          is_available: true,
          sort_order: detail?.sort_order || 0,
          prepare_time: detail?.prepare_time || 15,
          image_asset_id: detail?.image_asset_id || 0,
          image_preview_url: previewUrl
        }
      })
    } catch (err) {
      logger.error('Load dish edit page failed', err)
      this.setData({
        bootstrapped: false,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '菜品编辑页加载失败，请重试')
      })
    }
  },

  async refreshCategoriesSilently() {
    try {
      const categoryList = await DishManagementService.getDishCategories()
      const categoryOptions = buildCategoryOptions(categoryList)
      const categorySelection = resolveCategorySelection(
        categoryOptions,
        this.data.formData.category_id,
        this.data.selectedCategoryName,
        !this.data.isEdit && this.data.formData.category_id <= 0
      )

      this.setData({
        categoryOptions,
        selectedCategoryName: categorySelection.categoryName,
        selectedCategoryValue: categorySelection.categoryValue,
        'formData.category_id': categorySelection.categoryId
      })
    } catch (err) {
      logger.warn('Refresh categories silently failed', err)
    }
  },

  onRetry() {
    void this.loadPageData()
  },

  onManageCategories() {
    if (this.data.categoryCreateSubmitting) {
      return
    }

    this.setData({
      createPopupVisible: true,
      createPopupMode: 'category',
      createInputValue: ''
    })
  },

  onCloseCreatePopup() {
    if (this.data.categoryCreateSubmitting || this.data.tagSubmitting) {
      return
    }

    this.setData({
      createPopupVisible: false,
      createPopupMode: '',
      createInputValue: ''
    })
  },

  onCreateInputChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    this.setData({ createInputValue: (e.detail.value || '').replace(/^\s+/, '') })
  },

  async onConfirmCreatePopup() {
    const mode = this.data.createPopupMode
    if (!mode) {
      return
    }

    const name = this.data.createInputValue.trim()
    if (!name) {
      wx.showToast({ title: mode === 'category' ? '请输入分类名称' : '标签名称不能为空', icon: 'none' })
      return
    }

    if (mode === 'category') {
      if (this.data.categoryCreateSubmitting) {
        return
      }

      this.setData({ categoryCreateSubmitting: true })

      try {
        const created = await DishManagementService.createDishCategory({ name })
        const nextCategoryOptions = [...this.data.categoryOptions, {
          label: created.name,
          value: String(created.id)
        }]

        this.setData({
          createPopupVisible: false,
          createPopupMode: '',
          createInputValue: '',
          categoryOptions: nextCategoryOptions,
          selectedCategoryName: created.name,
          selectedCategoryValue: String(created.id),
          'formData.category_id': created.id
        })
      } catch (err) {
        logger.error('Create dish category failed', err)
        wx.showToast({ title: getErrorUserMessage(err, '新增分类失败，请重试'), icon: 'none' })
      } finally {
        this.setData({ categoryCreateSubmitting: false })
      }
      return
    }

    if (this.data.tagSubmitting) {
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
        createPopupVisible: false,
        createPopupMode: '',
        createInputValue: '',
        availableDishTags,
        selectedDishTagIds,
        selectedDishTagState: buildSelectedDishTagState(selectedDishTagIds),
        loadWarningMessage: ''
      })
    } catch (err) {
      logger.error('Create dish tag failed', err)
      wx.showToast({ title: '新增标签失败，请重试', icon: 'none' })
    } finally {
      this.setData({ tagSubmitting: false })
    }
  },

  onInputChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) {
      return
    }

    const { value } = e.detail
    if (field === 'prepare_time') {
      const prepareTime = Number.parseInt(value, 10)
      this.setData({ [`formData.${field}`]: Number.isFinite(prepareTime) ? prepareTime : 0 })
      return
    }

    this.setData({ [`formData.${field}`]: value.replace(/^\s+/, '') })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: !!e.detail.value })
  },

  onFeaturedTagToggle(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { tag } = e.currentTarget.dataset as { tag?: string }
    if (tag === '推荐') {
      this.setData({ isFeatured: !!e.detail.value })
      return
    }
    if (tag === '热卖') {
      this.setData({ isHotSelling: !!e.detail.value })
    }
  },

  onDishTagChange(e: WechatMiniprogram.CustomEvent<{ value: string[] }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedDishTagIds = values
      .map((value) => Number(value))
      .filter((id) => Number.isFinite(id) && id > 0)

    this.setData({
      selectedDishTagIds,
      selectedDishTagState: buildSelectedDishTagState(selectedDishTagIds)
    })
  },

  onDishTagToggle(e: WechatMiniprogram.CustomEvent) {
    const { tagId } = e.currentTarget.dataset as { tagId?: number }
    if (typeof tagId !== 'number') {
      return
    }

    const detail = e.detail as boolean | { checked?: boolean } | undefined
    const nextChecked = typeof detail === 'boolean'
      ? detail
      : !!detail?.checked

    const selectedDishTagIds = nextChecked
      ? (this.data.selectedDishTagIds.includes(tagId)
        ? this.data.selectedDishTagIds
        : [...this.data.selectedDishTagIds, tagId])
      : this.data.selectedDishTagIds.filter((id) => id !== tagId)

    this.setData({
      selectedDishTagIds,
      selectedDishTagState: buildSelectedDishTagState(selectedDishTagIds)
    })
  },

  onCreateDishTag() {
    if (this.data.tagSubmitting) {
      return
    }

    this.setData({
      createPopupVisible: true,
      createPopupMode: 'tag',
      createInputValue: ''
    })
  },

  onAddCustomizationGroup() {
    const editor = this.selectComponent('#customizationEditor') as { openAddGroupDialog?: () => void } | null
    editor?.openAddGroupDialog?.()
  },

  onPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const value = e.detail.value.trim()
    const parsedPrice = Number.parseFloat(value)
    this.setData({
      displayPrice: value,
      'formData.price': Number.isFinite(parsedPrice) && parsedPrice > 0 ? Math.round(parsedPrice * 100) : 0
    })
  },

  onMemberPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const value = e.detail.value.trim()
    const parsedPrice = Number.parseFloat(value)
    this.setData({
      displayMemberPrice: value,
      'formData.member_price': Number.isFinite(parsedPrice) && parsedPrice > 0 ? Math.round(parsedPrice * 100) : 0
    })
  },

  applyPersistedDishState(dish: DishResponse, fallbackPreviewUrl = '') {
    const previewUrl = toDishPreviewUrl(dish.image_url) || fallbackPreviewUrl
    const imageAssetId = dish.image_asset_id || 0

    this.setData({
      dishId: dish.id || this.data.dishId,
      isEdit: true,
      persistedImageAssetId: imageAssetId,
      persistedImagePreviewUrl: previewUrl,
      'formData.image_asset_id': imageAssetId,
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
    const urls = this.data.fileList.map((item) => item.url).filter((url) => !!url)
    if (!urls.length) {
      return
    }

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

  showCategoryPicker() {
    if (!this.data.categoryOptions.length) {
      wx.showToast({ title: '暂无分类，请先添加分类', icon: 'none' })
      return
    }

    this.setData({ categoryVisible: true })
  },

  onCategoryConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null, label: string[] | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const labels = Array.isArray(e.detail?.label) ? e.detail.label : []
    const selectedValue = String(values[0] ?? this.data.selectedCategoryValue ?? '')
    const categoryId = Number(selectedValue || this.data.formData.category_id)
    const categoryName = String(labels[0] ?? this.data.selectedCategoryName ?? '')

    if (!Number.isFinite(categoryId) || categoryId <= 0) {
      this.setData({ categoryVisible: false })
      wx.showToast({ title: '请选择分类', icon: 'none' })
      return
    }

    this.setData({
      'formData.category_id': categoryId,
      selectedCategoryValue: selectedValue,
      selectedCategoryName: categoryName,
      categoryVisible: false
    })
  },

  onCategoryCancel() {
    this.setData({ categoryVisible: false })
  },

  onCustomizationGroupsChange(e: WechatMiniprogram.CustomEvent<{ value?: CustomizationGroupDraft[] }>) {
    const customizationGroups = Array.isArray(e.detail?.value) ? e.detail.value : []
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
      if (tagIdByName.has(optionName)) {
        continue
      }
      const created = await TagService.createTag({ name: optionName, type: 'customization' })
      tagIdByName.set(created.name, created.id)
    }

    return normalizedGroups.map((group, groupIndex) => ({
      name: group.name,
      is_required: true,
      sort_order: groupIndex,
      options: group.options.map((option, optionIndex) => ({
        tag_id: tagIdByName.get(option.name) || 0,
        extra_price: option.extraPriceYuan ? Math.round(Number(option.extraPriceYuan) * 100) : 0,
        sort_order: optionIndex
      }))
    }))
  },

  validateBeforeSubmit(): string {
    const { formData, categoryOptions } = this.data
    if (!formData.name.trim()) {
      return '请输入菜品名称'
    }
    if (formData.category_id <= 0) {
      return categoryOptions.length > 0 ? '请选择菜品分类' : '请先创建菜品分类'
    }
    if (formData.price <= 0) {
      return '请输入正确价格'
    }
    if (formData.member_price > 0 && formData.member_price >= formData.price) {
      return '会员价需小于售价'
    }
    if (formData.prepare_time < 1 || formData.prepare_time > 120) {
      return '出餐时间需在1-120分钟'
    }
    if (this.data.imageUploading) {
      return '请等待图片上传完成'
    }
    return ''
  },

  buildSubmitPayload(categoryId: number): CreateDishRequest | UpdateDishRequest {
    const name = this.data.formData.name.trim()
    const description = this.data.formData.description.trim()
    const payload: CreateDishRequest | UpdateDishRequest = {
      name,
      category_id: categoryId,
      price: this.data.formData.price,
      is_online: this.data.formData.is_online,
      is_available: true,
      prepare_time: this.data.formData.prepare_time
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
    if (this.data.submitting || this.data.initialLoading) {
      return
    }

    const validationMessage = this.validateBeforeSubmit()
    if (validationMessage) {
      wx.showToast({ title: validationMessage, icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    let currentDishId = this.data.dishId
    let baseDishSaved = false

    try {
      const categoryId = this.data.formData.category_id
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