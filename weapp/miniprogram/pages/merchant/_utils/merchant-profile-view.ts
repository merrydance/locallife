import {
  MerchantCategoryTag,
  MerchantOperatorResponse
} from '../../../api/merchant'

export interface MerchantProfileForm {
  name: string
  phone: string
  address: string
  description: string
  latitude: string
  longitude: string
}

export interface LocationViewState {
  hasLocation: boolean
  addressDisplay: string
  latitudeDisplay: string
  longitudeDisplay: string
  coordinateSummary: string
  locationHint: string
}

export interface TagItem extends MerchantCategoryTag {
  selected: boolean
}

export interface CategoryPickerOption {
  label: string
  value: string
}

interface RequestErrorWithStatus {
  statusCode?: unknown
  code?: unknown
}

export const EMPTY_FORM: MerchantProfileForm = {
  name: '',
  phone: '',
  address: '',
  description: '',
  latitude: '',
  longitude: ''
}

export const PROFILE_AUTO_REFRESH_WINDOW_MS = 60 * 1000

export function buildCategorySelectionState(allTags: MerchantCategoryTag[], selectedTags: MerchantCategoryTag[]) {
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

export function getSelectedCategoryIds(tags: TagItem[]) {
  return tags
    .filter((tag) => tag.selected)
    .map((tag) => tag.id)
}

export function hasCategorySelectionChanged(currentTags: TagItem[], persistedTagIds: number[]) {
  const currentSelectedIds = getSelectedCategoryIds(currentTags).sort((left, right) => left - right)
  const lastSavedIds = [...persistedTagIds].sort((left, right) => left - right)

  if (currentSelectedIds.length !== lastSavedIds.length) {
    return true
  }

  return currentSelectedIds.some((id, index) => id !== lastSavedIds[index])
}

export function normalizeCategoryIds(values: Array<string | number>) {
  return values
    .map((value) => Number(value))
    .filter((value) => Number.isInteger(value) && value > 0)
}

export function buildCategorySelectionPatch(tags: TagItem[], selectedIds: number[], persistedTagIds: number[]) {
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

export function buildCategoryViewState(tags: TagItem[], persistedTagIds: number[]) {
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

export function buildChosenLocationAddress(result: WechatMiniprogram.ChooseLocationSuccessCallbackResult) {
  const address = result.address || ''
  const name = result.name || ''
  if (address && name) {
    return address.includes(name) ? address : `${address} ${name}`
  }
  return address || name || ''
}

export function buildProfileForm(profile: MerchantOperatorResponse): MerchantProfileForm {
  return {
    name: profile.name || '',
    phone: profile.phone || '',
    address: profile.address || '',
    description: profile.description || '',
    latitude: profile.latitude || '',
    longitude: profile.longitude || ''
  }
}

export function buildLocationViewState(form: MerchantProfileForm): LocationViewState {
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

export function hasFormChanged(current: MerchantProfileForm, initial: MerchantProfileForm) {
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

export function validateBeforeSubmit(form: MerchantProfileForm, initialForm: MerchantProfileForm) {
  return validateName(form.name)
    || validatePhone(form.phone, initialForm.phone)
    || validateDescription(form.description)
    || validateLocation(form)
    || ''
}

export function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
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

export function isVersionConflictError(error: unknown): boolean {
  return getErrorStatusCode(error) === 409
}

export function confirmClearMerchantCategories() {
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