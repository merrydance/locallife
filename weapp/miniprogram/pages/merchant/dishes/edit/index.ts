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
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { settleAll, isSettledFulfilled } from '../../../../utils/promise'
import {
  buildCategoryCreateSuccessPatch,
  buildCategoryOptions,
  buildDishCategoryRefreshPatch,
  buildDishCustomizationPayload,
  buildDishEditLoadPatch,
  buildDishFeaturedTags,
  buildDishFileList,
  buildDishPersistedStatePatch,
  buildDishSubmitPayload,
  buildDishTagCreateSuccessPatch,
  buildSelectedDishTagState,
  cloneUploadFileList,
  CustomizationGroupDraft,
  CreatePopupMode,
  CategoryOption,
  DishEditPageOptions,
  extractSelectedDishTagIds,
  FormInputDetail,
  getDishEditValidationMessage,
  isDishFeatureTagName,
  mapCustomizationGroupsToDrafts,
  mergeSelectableDishTags,
  parseCurrencyInput,
  resolveCategorySelection,
  resolveDishCategoryConfirmResult,
  resolveDishImageRemoveResult,
  UploadFileItem
} from '../../../../utils/merchant-dish-edit-view'

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
      is_packaging: false,
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
      const customizationGroups = this.data.isEdit
        ? mapCustomizationGroupsToDrafts(
          isSettledFulfilled(customizationResult) ? customizationResult.value : detail?.customization_groups
        )
        : []

      this.setData(buildDishEditLoadPatch({
        isEdit: this.data.isEdit,
        detail,
        categoryOptions,
        currentCategoryId: this.data.formData.category_id,
        currentCategoryName: this.data.selectedCategoryName,
        availableDishTags,
        selectedDishTagIds,
        customizationGroups,
        warningMessages
      }))
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
      this.setData(buildDishCategoryRefreshPatch({
        categoryOptions,
        currentCategoryId: this.data.formData.category_id,
        currentCategoryName: this.data.selectedCategoryName,
        allowDefaultSelection: !this.data.isEdit && this.data.formData.category_id <= 0
      }))
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
        this.setData(buildCategoryCreateSuccessPatch(this.data.categoryOptions, created))
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
    if (isDishFeatureTagName(name)) {
      wx.showToast({ title: '推荐和热卖请使用下方排序标签开关', icon: 'none' })
      return
    }

    this.setData({ tagSubmitting: true })
    try {
      const created = await TagService.createTag({ name, type: 'dish' })
      this.setData(buildDishTagCreateSuccessPatch(this.data.availableDishTags, this.data.selectedDishTagIds, created))
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

    if (field === 'is_packaging') {
      const isPackaging = !!e.detail.value
      this.setData({
        'formData.is_packaging': isPackaging,
        'formData.is_online': isPackaging ? true : this.data.formData.is_online,
        'formData.is_available': true
      })
      return
    }

    if (field === 'is_online' && this.data.formData.is_packaging) {
      this.setData({ 'formData.is_online': true })
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
    const parsedPrice = parseCurrencyInput(value)
    this.setData({
      displayPrice: value,
      'formData.price': parsedPrice.isValid ? parsedPrice.cents : 0
    })
  },

  onMemberPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const value = e.detail.value.trim()
    const parsedPrice = parseCurrencyInput(value)
    this.setData({
      displayMemberPrice: value,
      'formData.member_price': parsedPrice.isValid ? parsedPrice.cents : 0
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

    const previousFileList = cloneUploadFileList(this.data.fileList)
    const previousImageAssetId = this.data.formData.image_asset_id
    const previousImagePreviewUrl = this.data.formData.image_preview_url

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
      this.setData({
        fileList: previousFileList.length ? previousFileList : buildDishFileList(previousImagePreviewUrl),
        'formData.image_asset_id': previousImageAssetId,
        'formData.image_preview_url': previousImagePreviewUrl
      })
      wx.showToast({ title: getErrorUserMessage(err, '上传失败，请重试'), icon: 'none' })
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
    const result = resolveDishImageRemoveResult({
      isEdit: this.data.isEdit,
      persistedImageAssetId: this.data.persistedImageAssetId,
      persistedImagePreviewUrl: this.data.persistedImagePreviewUrl
    })
    this.setData(result.patch)
    if (result.toastMessage) {
      wx.showToast({ title: result.toastMessage, icon: 'none' })
    }
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
    const result = resolveDishCategoryConfirmResult({
      values,
      labels,
      selectedCategoryValue: this.data.selectedCategoryValue,
      selectedCategoryName: this.data.selectedCategoryName,
      currentCategoryId: this.data.formData.category_id
    })
    this.setData(result.patch)
    if (result.errorMessage) {
      wx.showToast({ title: result.errorMessage, icon: 'none' })
    }
  },

  onCategoryCancel() {
    this.setData({ categoryVisible: false })
  },

  onCustomizationGroupsChange(e: WechatMiniprogram.CustomEvent<{ value?: CustomizationGroupDraft[] }>) {
    const customizationGroups = Array.isArray(e.detail?.value) ? e.detail.value : []
    this.setData({ customizationGroups })
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading) {
      return
    }

    const validationMessage = getDishEditValidationMessage({
      formData: this.data.formData,
      categoryOptions: this.data.categoryOptions,
      displayPrice: this.data.displayPrice,
      displayMemberPrice: this.data.displayMemberPrice,
      imageUploading: this.data.imageUploading
    })
    if (validationMessage) {
      wx.showToast({ title: validationMessage, icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    const startedAsEdit = this.data.isEdit
    let currentDishId = this.data.dishId
    let baseDishSaved = false

    try {
      const featuredTags = buildDishFeaturedTags({
        isFeatured: this.data.isFeatured,
        isHotSelling: this.data.isHotSelling
      })
      const payload = buildDishSubmitPayload({
        formData: this.data.formData,
        selectedDishTagIds: this.data.selectedDishTagIds,
        isEdit: this.data.isEdit
      })

      if (startedAsEdit) {
        const updatedDish = await DishManagementService.updateDish(this.data.dishId, payload as UpdateDishRequest)
        currentDishId = this.data.dishId
        baseDishSaved = true
        this.setData(buildDishPersistedStatePatch({
          dish: updatedDish,
          dishId: this.data.dishId,
          fallbackPreviewUrl: this.data.formData.image_preview_url,
          selectedCategoryName: this.data.selectedCategoryName,
          selectedCategoryValue: this.data.selectedCategoryValue
        }))
      } else {
        const createdDish = await DishManagementService.createDish(payload as CreateDishRequest)
        currentDishId = createdDish.id
        baseDishSaved = true
        this.setData(buildDishPersistedStatePatch({
          dish: createdDish,
          dishId: this.data.dishId,
          fallbackPreviewUrl: this.data.formData.image_preview_url,
          selectedCategoryName: this.data.selectedCategoryName,
          selectedCategoryValue: this.data.selectedCategoryValue
        }))
      }

      if (currentDishId > 0) {
        await DishManagementService.setDishFeaturedTags(currentDishId, featuredTags)
      }

      const customizationGroups = await buildDishCustomizationPayload(this.data.customizationGroups)
      if (currentDishId > 0 && (startedAsEdit || customizationGroups.length > 0)) {
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