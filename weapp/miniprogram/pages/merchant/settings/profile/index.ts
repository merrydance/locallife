import {
  getAvailableMerchantTags,
  getMyMerchantProfile,
  getMyMerchantTags,
  MerchantCategoryTag,
  MerchantOperatorResponse,
  setMyMerchantTags,
  updateMyMerchantProfile
} from '../../../../api/merchant'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

interface MerchantProfileForm {
  name: string
  phone: string
  address: string
  description: string
  latitude: string
  longitude: string
}

interface LocationViewState {
  hasLocation: boolean
  addressDisplay: string
  latitudeDisplay: string
  longitudeDisplay: string
  coordinateSummary: string
  locationHint: string
}

interface TagItem extends MerchantCategoryTag {
  selected: boolean
}

interface CategoryPickerOption {
  label: string
  value: string
}

interface RequestErrorWithStatus {
  statusCode?: unknown
  code?: unknown
}

const EMPTY_FORM: MerchantProfileForm = {
  name: '',
  phone: '',
  address: '',
  description: '',
  latitude: '',
  longitude: ''
}

const PROFILE_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function buildCategorySelectionState(allTags: MerchantCategoryTag[], selectedTags: MerchantCategoryTag[]) {
  const selectedIds = new Set((selectedTags || []).map((tag) => tag.id))

  return {
    tags: (allTags || []).map((tag) => ({
      ...tag,
      selected: selectedIds.has(tag.id)
    })),
    selectedCount: selectedIds.size,
    persistedTagIds: [...selectedIds]
  }
}

function getSelectedCategoryIds(tags: TagItem[]) {
  return tags
    .filter((tag) => tag.selected)
    .map((tag) => tag.id)
}

function hasCategorySelectionChanged(currentTags: TagItem[], persistedTagIds: number[]) {
  const currentSelectedIds = getSelectedCategoryIds(currentTags).sort((left, right) => left - right)
  const lastSavedIds = [...persistedTagIds].sort((left, right) => left - right)

  if (currentSelectedIds.length !== lastSavedIds.length) {
    return true
  }

  return currentSelectedIds.some((id, index) => id !== lastSavedIds[index])
}

function normalizeCategoryIds(values: Array<string | number>) {
  return values
    .map((value) => Number(value))
    .filter((value) => Number.isInteger(value) && value > 0)
}

function buildCategorySelectionPatch(tags: TagItem[], selectedIds: number[], persistedTagIds: number[]) {
  const selectedIdSet = new Set(selectedIds)
  const nextTags = tags.map((tag) => ({
    ...tag,
    selected: selectedIdSet.has(tag.id)
  }))
  const nextSelectedIds = getSelectedCategoryIds(nextTags)
  const hasCategoryChanges = hasCategorySelectionChanged(nextTags, persistedTagIds)

  return {
    tags: nextTags,
    selectedCategoryIds: nextSelectedIds,
    selectedCategoryCount: nextSelectedIds.length,
    hasCategoryChanges
  }
}

function buildCategoryPickerOptions(tags: TagItem[]): CategoryPickerOption[] {
  return tags
    .filter((tag) => !tag.selected)
    .map((tag) => ({
      label: tag.name,
      value: String(tag.id)
    }))
}

function buildCategoryViewState(tags: TagItem[], persistedTagIds: number[]) {
  const selectedCategoryTags = tags.filter((tag) => tag.selected)
  const selectedCategoryIds = selectedCategoryTags.map((tag) => tag.id)
  const categoryPickerOptions = buildCategoryPickerOptions(tags)
  const hasCategoryChanges = hasCategorySelectionChanged(tags, persistedTagIds)

  return {
    tags,
    selectedCategoryTags,
    selectedCategoryIds,
    selectedCategoryCount: selectedCategoryIds.length,
    categoryPickerOptions,
    categoryPickerValue: categoryPickerOptions[0]?.value || '',
    categoryPickerTriggerText: selectedCategoryIds.length >= 5
      ? '已选满 5 项'
      : categoryPickerOptions.length
        ? '选择类目'
        : selectedCategoryIds.length
          ? '无更多可选项'
          : '暂无可选类目',
    hasCategoryChanges
  }
}

function buildLocationHint(address: string, latitude?: string | null, longitude?: string | null) {
  const hasAddress = address.trim().length > 0
  const hasCoordinates = !!latitude && !!longitude

  if (hasAddress && hasCoordinates) {
    return '地图选点会同步更新经营地址、纬度和经度。'
  }

  if (hasAddress || hasCoordinates) {
    return '当前只保存了部分位置字段，请重新选择经营位置补齐。'
  }

  return '当前还没有经营位置，请通过地图选点写入地址和坐标。'
}

function buildChosenLocationAddress(result: WechatMiniprogram.ChooseLocationSuccessCallbackResult) {
  const address = result.address || ''
  const name = result.name || ''
  if (address && name) {
    return address.includes(name) ? address : `${address} ${name}`
  }
  return address || name || ''
}

function buildForm(profile: MerchantOperatorResponse): MerchantProfileForm {
  return {
    name: profile.name || '',
    phone: profile.phone || '',
    address: profile.address || '',
    description: profile.description || '',
    latitude: profile.latitude || '',
    longitude: profile.longitude || ''
  }
}

function buildLocationViewState(form: MerchantProfileForm): LocationViewState {
  const hasLocation = hasCompleteLocation(form)

  return {
    hasLocation,
    addressDisplay: form.address.trim() || '未设置经营地址',
    latitudeDisplay: form.latitude.trim() || '--',
    longitudeDisplay: form.longitude.trim() || '--',
    coordinateSummary: `${form.latitude.trim() || '--'} / ${form.longitude.trim() || '--'}`,
    locationHint: buildLocationHint(form.address, form.latitude, form.longitude)
  }
}

function hasFormChanged(current: MerchantProfileForm, initial: MerchantProfileForm) {
  return current.name !== initial.name
    || current.phone !== initial.phone
    || current.address !== initial.address
    || current.description !== initial.description
    || current.latitude !== initial.latitude
    || current.longitude !== initial.longitude
}

function validateName(name: string) {
  const trimmed = name.trim()

  if (!trimmed) {
    return '请输入店铺名称'
  }

  if (trimmed.length < 2) {
    return '店铺名称至少 2 个字'
  }

  if (trimmed.length > 50) {
    return '店铺名称最多 50 字'
  }

  return ''
}

function validatePhone(phone: string, initialPhone: string) {
  const trimmed = phone.trim()

  if (!trimmed) {
    return initialPhone.trim() ? '当前版本仅支持修改为 11 位手机号，不支持直接清空' : ''
  }

  if (!/^1[3-9]\d{9}$/.test(trimmed)) {
    return '请输入 11 位手机号'
  }

  return ''
}

function validateDescription(description: string) {
  if (description.trim().length > 500) {
    return '店铺介绍最多 500 字'
  }

  return ''
}

function validateLocation(form: MerchantProfileForm) {
  const address = form.address.trim()
  const latitude = form.latitude.trim()
  const longitude = form.longitude.trim()

  if (hasPartialLocation(form) && !hasCompleteLocation(form)) {
    return '请通过“选择经营位置”补齐地址和坐标'
  }

  if (address && address.length < 5) {
    return '请选择更准确的经营位置'
  }

  if (latitude && !isCoordinateInRange(latitude, 3, 54)) {
    return '纬度需在 3.0 到 54.0 之间'
  }

  if (longitude && !isCoordinateInRange(longitude, 73, 135)) {
    return '经度需在 73.0 到 135.0 之间'
  }

  return ''
}

function validateBeforeSubmit(form: MerchantProfileForm, initialForm: MerchantProfileForm) {
  return validateName(form.name)
    || validatePhone(form.phone, initialForm.phone)
    || validateDescription(form.description)
    || validateLocation(form)
    || ''
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function isCoordinateInRange(value: string, min: number, max: number): boolean {
  const parsed = Number.parseFloat(value)
  return Number.isFinite(parsed) && parsed >= min && parsed <= max
}

function hasCompleteLocation(form: MerchantProfileForm) {
  return !!form.address.trim() && !!form.latitude.trim() && !!form.longitude.trim()
}

function hasPartialLocation(form: MerchantProfileForm) {
  return !!form.address.trim() || !!form.latitude.trim() || !!form.longitude.trim()
}

function getErrorStatusCode(error: unknown): number | undefined {
  if (!error || typeof error !== 'object') {
    return undefined
  }

  const knownError = error as RequestErrorWithStatus
  const candidates = [knownError.statusCode, knownError.code]

  for (const candidate of candidates) {
    const numericCode = typeof candidate === 'number' ? candidate : Number(candidate)
    if (Number.isFinite(numericCode)) {
      return numericCode >= 40900 && numericCode < 41000 ? 409 : numericCode
    }
  }

  return undefined
}

function isVersionConflictError(error: unknown): boolean {
  return getErrorStatusCode(error) === 409
}

function confirmClearMerchantCategories() {
  return new Promise<boolean>((resolve) => {
    wx.showModal({
      title: '确认清除类目？',
      content: '未选择任何类目将导致店铺不出现在分类筛选中，继续吗？',
      confirmText: '确认清除',
      cancelText: '取消',
      success: (result) => resolve(!!result.confirm),
      fail: () => resolve(false)
    })
  })
}

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
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    categoryLoading: false,
    lastLoadedAt: 0,
    version: 0,
    hasLocation: false,
    addressDisplay: '未设置经营地址',
    latitudeDisplay: '--',
    longitudeDisplay: '--',
    coordinateSummary: '未设置',
    locationHint: '当前还没有经营位置，请通过地图选点写入地址和坐标。',
    categoryErrorMessage: '',
    tags: [] as TagItem[],
    selectedCategoryTags: [] as TagItem[],
    selectedCategoryIds: [] as number[],
    selectedCategoryCount: 0,
    persistedCategoryIds: [] as number[],
    categoryPickerVisible: false,
    categoryPickerOptions: [] as CategoryPickerOption[],
    categoryPickerValue: '',
    categoryPickerTriggerText: '暂无可选类目',
    form: { ...EMPTY_FORM } as MerchantProfileForm,
    initialForm: { ...EMPTY_FORM } as MerchantProfileForm,
    hasProfileChanges: false,
    hasCategoryChanges: false,
    hasChanges: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    this.loadProfile(true, true)
    this.loadCategories()
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      if (shouldAutoRefresh(this.data.lastLoadedAt, PROFILE_AUTO_REFRESH_WINDOW_MS)) {
        this.loadProfile(false)
        this.loadCategories()
      }
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }
    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      wx.showToast({ title: '当前有未保存修改，请先保存后再刷新', icon: 'none' })
      return
    }
    this.loadProfile(false, true)
    this.loadCategories()
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadProfile(false, true)
    this.loadCategories(true)
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

  async loadProfile(showLoading = true, force = false) {
    if (this.data.loading) return

    const hasExistingData = !this.data.initialLoading
    const isSilentRefresh = !showLoading && hasExistingData

    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastLoadedAt, PROFILE_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const profile = await getMyMerchantProfile()
      const form = buildForm(profile)
      const locationView = buildLocationViewState(form)
      this.setData({
        version: profile.version,
        ...locationView,
        form,
        initialForm: { ...form },
        hasProfileChanges: false,
        hasChanges: this.data.hasCategoryChanges,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err: unknown) {
      logger.error('Load merchant profile settings failed', err)
      const message = getErrorMessage(err, '店铺资料加载失败，请重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  async loadCategories(showToastOnError = false) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }

    if (this.data.categoryLoading) {
      return
    }

    this.setData({
      categoryLoading: true,
      categoryErrorMessage: ''
    })

    try {
      const [currentRes, allRes] = await Promise.all([
        getMyMerchantTags(),
        getAvailableMerchantTags()
      ])

      const nextState = buildCategorySelectionState(allRes.tags || [], currentRes.tags || [])
      const nextViewState = buildCategoryViewState(nextState.tags, nextState.persistedTagIds)
      this.setData({
        ...nextViewState,
        persistedCategoryIds: nextState.persistedTagIds,
        categoryPickerVisible: false,
        categoryErrorMessage: '',
        hasCategoryChanges: false,
        hasChanges: this.data.hasProfileChanges
      })
    } catch (err: unknown) {
      logger.error('Load merchant profile categories failed', err)
      const message = getErrorMessage(err, '经营类目加载失败，请重试')

      this.setData({
        categoryErrorMessage: this.data.tags.length > 0 ? `${message}，当前已保留上次同步结果` : message
      })

      if (showToastOnError && !this.data.initialLoading) {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ categoryLoading: false })
    }
  },

  onInputChange(
    e: WechatMiniprogram.CustomEvent<{ value: string }> & { currentTarget: { dataset: { field: keyof MerchantProfileForm } } }
  ) {
    const field = e.currentTarget.dataset.field
    const nextForm = {
      ...this.data.form,
      [field]: e.detail.value
    }
    const hasProfileChanges = hasFormChanged(nextForm, this.data.initialForm)
    this.setData({
      refreshErrorMessage: '',
      form: nextForm,
      hasProfileChanges,
      hasChanges: hasProfileChanges || this.data.hasCategoryChanges
    })
  },

  onOpenCategoryPicker() {
    if (this.data.categoryLoading || this.data.loading || this.data.saving) {
      return
    }

    if (this.data.selectedCategoryCount >= 5) {
      wx.showToast({ title: '最多选 5 个类目', icon: 'none' })
      return
    }

    if (!this.data.categoryPickerOptions.length) {
      wx.showToast({ title: this.data.selectedCategoryCount ? '没有更多可选类目' : '暂无可选类目', icon: 'none' })
      return
    }

    this.setData({
      categoryPickerVisible: true,
      categoryPickerValue: this.data.categoryPickerOptions[0]?.value || ''
    })
  },

  onCloseCategoryPicker() {
    this.setData({ categoryPickerVisible: false })
  },

  onCategoryPickerConfirm(
    e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null, label: string[] | null }>
  ) {
    if (this.data.categoryLoading || this.data.loading || this.data.saving) {
      return
    }

    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const pickedId = normalizeCategoryIds(values)[0]
    if (!pickedId) {
      this.setData({ categoryPickerVisible: false })
      return
    }

    const nextSelectedIds = [...this.data.selectedCategoryIds]
    if (!nextSelectedIds.includes(pickedId)) {
      if (nextSelectedIds.length >= 5) {
        wx.showToast({ title: '最多选 5 个类目', icon: 'none' })
        this.setData({ categoryPickerVisible: false })
        return
      }
      nextSelectedIds.push(pickedId)
    }

    const nextPatch = buildCategorySelectionPatch(this.data.tags, nextSelectedIds, this.data.persistedCategoryIds)
    const nextViewState = buildCategoryViewState(nextPatch.tags, this.data.persistedCategoryIds)
    this.setData({
      categoryErrorMessage: '',
      categoryPickerVisible: false,
      ...nextViewState,
      hasChanges: this.data.hasProfileChanges || nextViewState.hasCategoryChanges
    })
  },

  onRemoveCategory(e: WechatMiniprogram.TouchEvent) {
    if (this.data.categoryLoading || this.data.loading || this.data.saving) {
      return
    }

    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const nextSelectedIds = this.data.selectedCategoryIds.filter((selectedId) => selectedId !== id)
    const nextPatch = buildCategorySelectionPatch(this.data.tags, nextSelectedIds, this.data.persistedCategoryIds)
    const nextViewState = buildCategoryViewState(nextPatch.tags, this.data.persistedCategoryIds)
    this.setData({
      categoryErrorMessage: '',
      ...nextViewState,
      hasChanges: this.data.hasProfileChanges || nextViewState.hasCategoryChanges
    })
  },

  onRetryCategories() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadCategories(true)
  },

  validateForm() {
    const validationMessage = validateBeforeSubmit(this.data.form, this.data.initialForm)

    if (validationMessage) {
      wx.showToast({ title: validationMessage, icon: 'none' })
      return false
    }

    return true
  },

  onChooseLocation() {
    if (this.data.loading || this.data.saving) {
      return
    }

    wx.chooseLocation({
      success: (result) => {
        const fullAddress = buildChosenLocationAddress(result).trim()
        if (!fullAddress) {
          wx.showToast({ title: '未获取到完整地址，请重新选择', icon: 'none' })
          return
        }

        const nextForm = {
          ...this.data.form,
          address: fullAddress,
          latitude: String(result.latitude),
          longitude: String(result.longitude)
        }
        const locationView = buildLocationViewState(nextForm)
        const hasProfileChanges = hasFormChanged(nextForm, this.data.initialForm)

        this.setData({
          refreshErrorMessage: '',
          ...locationView,
          form: nextForm,
          hasProfileChanges,
          hasChanges: hasProfileChanges || this.data.hasCategoryChanges
        })
      },
      fail: (error) => {
        if (typeof error?.errMsg === 'string' && error.errMsg.includes('auth deny')) {
          wx.showModal({
            title: '需要位置权限',
            content: '请在设置中开启位置权限后再选择经营位置。',
            confirmText: '去设置',
            success: (result) => {
              if (result.confirm) {
                wx.openSetting()
              }
            }
          })
          return
        }

        if (typeof error?.errMsg === 'string' && error.errMsg.includes('cancel')) {
          return
        }

        wx.showToast({ title: '位置选择失败，请稍后重试', icon: 'none' })
      }
    })
  },

  async navigateBackToPreviousPage(shouldRefreshPrevious = false) {
    const pages = getCurrentPages()
    const prevPage = pages[pages.length - 2] as { refreshAll?: (showLoading?: boolean) => Promise<void> | void } | undefined

    if (shouldRefreshPrevious && prevPage?.refreshAll) {
      await prevPage.refreshAll(false)
    }

    wx.navigateBack()
  },

  async onSave() {
    if (this.data.saving || this.data.initialLoading) return

    if (!this.data.hasChanges) {
      await this.navigateBackToPreviousPage(false)
      return
    }

    if (!this.validateForm()) return

    const hadProfileChanges = this.data.hasProfileChanges
    const hadCategoryChanges = this.data.hasCategoryChanges
    const selectedCategoryIds = getSelectedCategoryIds(this.data.tags)

    if (hadCategoryChanges && selectedCategoryIds.length === 0) {
      const shouldContinue = await confirmClearMerchantCategories()
      if (!shouldContinue) {
        return
      }
    }

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

    let profileSaved = false

    try {
      if (hadProfileChanges) {
        const latitude = this.data.form.latitude.trim()
        const longitude = this.data.form.longitude.trim()

        const payload = {
          version: this.data.version,
          name: this.data.form.name.trim(),
          phone: this.data.form.phone.trim() || undefined,
          address: this.data.form.address.trim(),
          description: this.data.form.description.trim(),
          latitude: latitude || undefined,
          longitude: longitude || undefined
        }
        const updated = await updateMyMerchantProfile(payload)
        const form = buildForm(updated)
        const locationView = buildLocationViewState(form)
        this.setData({
          version: updated.version,
          refreshErrorMessage: '',
          ...locationView,
          form,
          initialForm: { ...form },
          hasProfileChanges: false,
          hasCategoryChanges: hadCategoryChanges,
          hasChanges: hadCategoryChanges,
          lastLoadedAt: Date.now()
        })

        try {
          const currentMerchant = wx.getStorageSync('current_merchant') || {}
          wx.setStorageSync('current_merchant', {
            ...currentMerchant,
            id: updated.id,
            merchant_id: updated.id,
            name: updated.name
          })
        } catch (storageErr) {
          logger.warn('Sync merchant profile cache failed', storageErr)
        }

        profileSaved = true
      }

      if (hadCategoryChanges) {
        const response = await setMyMerchantTags(selectedCategoryIds)
        const nextCategoryState = buildCategorySelectionState(this.data.tags, response.tags || [])
        const nextViewState = buildCategoryViewState(nextCategoryState.tags, nextCategoryState.persistedTagIds)
        this.setData({
          ...nextViewState,
          persistedCategoryIds: nextCategoryState.persistedTagIds,
          categoryPickerVisible: false,
          categoryErrorMessage: '',
          hasCategoryChanges: false,
          hasChanges: false,
          lastLoadedAt: Date.now()
        })
      }

      await this.navigateBackToPreviousPage(true)
    } catch (err: unknown) {
      if (hadProfileChanges && isVersionConflictError(err)) {
        await this.loadProfile(false, true)
        wx.showToast({ title: '资料已被其他操作更新，已刷新到最新内容', icon: 'none' })
        return
      }

      if (profileSaved && hadCategoryChanges) {
        logger.error('Save merchant profile categories failed after profile update', err)
        wx.showToast({ title: '基础资料已保存，经营类目保存失败，请重试', icon: 'none' })
        return
      }

      logger.error('Save merchant profile settings failed', err)
      const message = getErrorMessage(err, hadCategoryChanges ? '经营类目保存失败，请稍后重试' : '保存失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadProfile(true, true)
    this.loadCategories(true)
  }
})