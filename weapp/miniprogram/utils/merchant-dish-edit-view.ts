import {
  CreateDishRequest,
  CustomizationGroup,
  CustomizationGroupInput,
  DishCategory,
  DishResponse,
  TagInfo,
  UpdateDishRequest,
  TagService
} from '../api/dish'
import { getPublicImageUrl } from './image'

export interface UploadFileItem {
  url: string
  status?: 'loading' | 'done' | 'failed'
  remotePath?: string
}

export interface DishEditPageOptions {
  id?: string
}

export interface FormInputDetail {
  value: string
}

export interface CategoryOption {
  label: string
  value: string
}

export interface CustomizationOptionDraft {
  key: string
  name: string
  extraPriceYuan: string
}

export interface CustomizationGroupDraft {
  key: string
  name: string
  is_required: boolean
  options: CustomizationOptionDraft[]
}

export interface DishEditFormData {
  name: string
  description: string
  category_id: number
  price: number
  member_price: number
  is_online: boolean
  is_available: boolean
  is_packaging: boolean
  sort_order: number
  prepare_time: number
  image_asset_id: number
  image_preview_url: string
}

interface CurrencyInputState {
  cents: number
  hasValue: boolean
  isValid: boolean
}

export type CreatePopupMode = 'category' | 'tag' | ''

const FEATURED_TAG_NAMES = new Set(['推荐', '热卖'])

export function parseCurrencyInput(value: string): CurrencyInputState {
  const trimmedValue = value.trim()
  if (!trimmedValue) {
    return { cents: 0, hasValue: false, isValid: false }
  }

  if (!/^-?\d+(\.\d{0,2})?$/.test(trimmedValue)) {
    return { cents: 0, hasValue: true, isValid: false }
  }

  const parsedValue = Number(trimmedValue)
  if (!Number.isFinite(parsedValue)) {
    return { cents: 0, hasValue: true, isValid: false }
  }

  return {
    cents: Math.round(parsedValue * 100),
    hasValue: true,
    isValid: true
  }
}

export function mapCustomizationGroupsToDrafts(groups?: CustomizationGroup[] | null): CustomizationGroupDraft[] {
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

export function toDishPreviewUrl(imageUrl?: string | null): string {
  return imageUrl ? getPublicImageUrl(imageUrl) : ''
}

export function buildDishFileList(previewUrl: string): UploadFileItem[] {
  return previewUrl ? [{ url: previewUrl, status: 'done' }] : []
}

export function buildCategoryOptions(list: DishCategory[]): CategoryOption[] {
  return list.map((category) => ({
    label: category.name,
    value: String(category.id)
  }))
}

export function resolveCategorySelection(
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

export function mergeSelectableDishTags(primaryTags: TagInfo[], fallbackTags: TagInfo[]): TagInfo[] {
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

export function extractSelectedDishTagIds(tags?: TagInfo[] | null): number[] {
  return (Array.isArray(tags) ? tags : [])
    .filter((tag) => !FEATURED_TAG_NAMES.has(tag.name))
    .map((tag) => tag.id)
    .filter((id) => Number.isFinite(id) && id > 0)
}

export function joinWarningMessages(messages: string[]): string {
  return messages.filter((message) => !!message).join('；')
}

export function buildSelectedDishTagState(selectedDishTagIds: number[]): Record<string, boolean> {
  return selectedDishTagIds.reduce<Record<string, boolean>>((result, id) => {
    result[String(id)] = true
    return result
  }, {})
}

export function cloneUploadFileList(fileList: UploadFileItem[]): UploadFileItem[] {
  return (Array.isArray(fileList) ? fileList : []).map((item) => ({ ...item }))
}

export function isDishFeatureTagName(name: string): boolean {
  return FEATURED_TAG_NAMES.has(name)
}

export function buildDishEditLoadPatch(params: {
  isEdit: boolean
  detail: DishResponse | null
  categoryOptions: CategoryOption[]
  currentCategoryId: number
  currentCategoryName: string
  availableDishTags: TagInfo[]
  selectedDishTagIds: number[]
  customizationGroups: CustomizationGroupDraft[]
  warningMessages: string[]
}) {
  const categorySelection = resolveCategorySelection(
    params.categoryOptions,
    params.detail?.category_id || params.currentCategoryId,
    params.detail?.category_name || params.currentCategoryName,
    !params.isEdit
  )
  const detailTags = Array.isArray(params.detail?.tags) ? params.detail.tags : []
  const previewUrl = toDishPreviewUrl(params.detail?.image_url)

  return {
    bootstrapped: true,
    initialLoading: false,
    initialError: false,
    initialErrorMessage: '',
    loadWarningMessage: joinWarningMessages(params.warningMessages),
    availableDishTags: params.availableDishTags,
    selectedDishTagIds: params.selectedDishTagIds,
    selectedDishTagState: buildSelectedDishTagState(params.selectedDishTagIds),
    categoryOptions: params.categoryOptions,
    selectedCategoryName: categorySelection.categoryName,
    selectedCategoryValue: categorySelection.categoryValue,
    persistedImageAssetId: params.detail?.image_asset_id || 0,
    persistedImagePreviewUrl: previewUrl,
    displayPrice: params.detail ? (params.detail.price / 100).toFixed(2) : '',
    displayMemberPrice: params.detail?.member_price ? (params.detail.member_price / 100).toFixed(2) : '',
    fileList: buildDishFileList(previewUrl),
    isFeatured: detailTags.some((tag) => tag.name === '推荐'),
    isHotSelling: detailTags.some((tag) => tag.name === '热卖'),
    customizationGroups: params.customizationGroups,
    formData: {
      name: params.detail?.name || '',
      description: params.detail?.description || '',
      category_id: categorySelection.categoryId,
      price: params.detail?.price || 0,
      member_price: params.detail?.member_price || 0,
      is_online: params.detail?.is_online ?? true,
      is_available: true,
      is_packaging: params.detail?.is_packaging ?? false,
      sort_order: params.detail?.sort_order || 0,
      prepare_time: params.detail?.prepare_time || 15,
      image_asset_id: params.detail?.image_asset_id || 0,
      image_preview_url: previewUrl
    }
  }
}

export function buildDishCategoryRefreshPatch(params: {
  categoryOptions: CategoryOption[]
  currentCategoryId: number
  currentCategoryName: string
  allowDefaultSelection: boolean
}) {
  const categorySelection = resolveCategorySelection(
    params.categoryOptions,
    params.currentCategoryId,
    params.currentCategoryName,
    params.allowDefaultSelection
  )

  return {
    categoryOptions: params.categoryOptions,
    selectedCategoryName: categorySelection.categoryName,
    selectedCategoryValue: categorySelection.categoryValue,
    'formData.category_id': categorySelection.categoryId
  }
}

export function buildCategoryCreateSuccessPatch(
  categoryOptions: CategoryOption[],
  created: DishCategory
) {
  return {
    createPopupVisible: false,
    createPopupMode: '' as CreatePopupMode,
    createInputValue: '',
    categoryOptions: [...categoryOptions, {
      label: created.name,
      value: String(created.id)
    }],
    selectedCategoryName: created.name,
    selectedCategoryValue: String(created.id),
    'formData.category_id': created.id
  }
}

export function buildDishTagCreateSuccessPatch(
  availableDishTags: TagInfo[],
  selectedDishTagIds: number[],
  created: TagInfo
) {
  const nextAvailableDishTags = availableDishTags.some((tag) => tag.id === created.id)
    ? availableDishTags
    : [...availableDishTags, created]
  const nextSelectedDishTagIds = selectedDishTagIds.includes(created.id)
    ? selectedDishTagIds
    : [...selectedDishTagIds, created.id]

  return {
    createPopupVisible: false,
    createPopupMode: '' as CreatePopupMode,
    createInputValue: '',
    availableDishTags: nextAvailableDishTags,
    selectedDishTagIds: nextSelectedDishTagIds,
    selectedDishTagState: buildSelectedDishTagState(nextSelectedDishTagIds),
    loadWarningMessage: ''
  }
}

export function buildDishPersistedStatePatch(params: {
  dish: DishResponse
  dishId: number
  fallbackPreviewUrl?: string
  selectedCategoryName: string
  selectedCategoryValue: string
}) {
  const previewUrl = toDishPreviewUrl(params.dish.image_url) || params.fallbackPreviewUrl || ''
  const imageAssetId = params.dish.image_asset_id || 0

  return {
    dishId: params.dish.id || params.dishId,
    isEdit: true,
    persistedImageAssetId: imageAssetId,
    persistedImagePreviewUrl: previewUrl,
    'formData.image_asset_id': imageAssetId,
    'formData.image_preview_url': previewUrl,
    'formData.category_id': params.dish.category_id || 0,
    fileList: buildDishFileList(previewUrl),
    selectedCategoryName: params.dish.category_name || params.selectedCategoryName,
    selectedCategoryValue: params.dish.category_id ? String(params.dish.category_id) : params.selectedCategoryValue
  }
}

export function resolveDishImageRemoveResult(params: {
  isEdit: boolean
  persistedImageAssetId: number
  persistedImagePreviewUrl: string
}) {
  if (params.isEdit && params.persistedImageAssetId > 0) {
    return {
      patch: {
        fileList: buildDishFileList(params.persistedImagePreviewUrl),
        'formData.image_asset_id': params.persistedImageAssetId,
        'formData.image_preview_url': params.persistedImagePreviewUrl
      },
      toastMessage: '当前暂不支持删除已发布图片，可重新上传替换'
    }
  }

  return {
    patch: {
      fileList: [],
      'formData.image_asset_id': 0,
      'formData.image_preview_url': ''
    },
    toastMessage: ''
  }
}

export function resolveDishCategoryConfirmResult(params: {
  values: Array<string | number>
  labels: string[]
  selectedCategoryValue: string
  selectedCategoryName: string
  currentCategoryId: number
}) {
  const selectedValue = String(params.values[0] ?? params.selectedCategoryValue ?? '')
  const categoryId = Number(selectedValue || params.currentCategoryId)
  const categoryName = String(params.labels[0] ?? params.selectedCategoryName ?? '')

  if (!Number.isFinite(categoryId) || categoryId <= 0) {
    return {
      patch: { categoryVisible: false },
      errorMessage: '请选择分类'
    }
  }

  return {
    patch: {
      'formData.category_id': categoryId,
      selectedCategoryValue: selectedValue,
      selectedCategoryName: categoryName,
      categoryVisible: false
    },
    errorMessage: ''
  }
}

export async function buildDishCustomizationPayload(
  customizationGroups: CustomizationGroupDraft[]
): Promise<CustomizationGroupInput[]> {
  const normalizedGroups = customizationGroups
    .map((group) => ({
      name: group.name.trim(),
      is_required: group.is_required,
      options: group.options.map((option) => ({
        name: option.name.trim(),
        extraPriceYuan: option.extraPriceYuan.trim()
      }))
    }))
    .map((group) => ({
      ...group,
      options: group.options.filter((option) => option.name || option.extraPriceYuan)
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
      if (!option.name) {
        throw new Error(`规格组“${group.name}”存在未填写名称的规格项`)
      }
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
}

export function getDishEditValidationMessage(params: {
  formData: DishEditFormData
  categoryOptions: CategoryOption[]
  displayPrice: string
  displayMemberPrice: string
  imageUploading: boolean
}) {
  const priceInput = parseCurrencyInput(params.displayPrice)
  const memberPriceInput = parseCurrencyInput(params.displayMemberPrice)

  if (!params.formData.name.trim()) {
    return '请输入菜品名称'
  }
  if (params.formData.category_id <= 0) {
    return params.categoryOptions.length > 0 ? '请选择菜品分类' : '请先创建菜品分类'
  }
  if (!priceInput.hasValue || !priceInput.isValid) {
    return '请输入正确价格'
  }
  if (priceInput.cents < 0) {
    return '价格不能为负数'
  }
  if (memberPriceInput.hasValue && !memberPriceInput.isValid) {
    return '请输入正确会员价'
  }
  if (memberPriceInput.isValid && memberPriceInput.cents < 0) {
    return '会员价不能为负数'
  }
  if (memberPriceInput.isValid && memberPriceInput.cents > 0 && memberPriceInput.cents >= priceInput.cents) {
    return '会员价需小于售价'
  }
  if (params.formData.prepare_time < 1 || params.formData.prepare_time > 120) {
    return '出餐时间需在1-120分钟'
  }
  if (params.formData.is_packaging && !params.formData.is_online) {
    return '包装菜品必须保持上架'
  }
  if (params.imageUploading) {
    return '请等待图片上传完成'
  }
  return ''
}

export function buildDishSubmitPayload(params: {
  formData: DishEditFormData
  selectedDishTagIds: number[]
  isEdit: boolean
}): CreateDishRequest | UpdateDishRequest {
  const name = params.formData.name.trim()
  const description = params.formData.description.trim()
  const payload: CreateDishRequest | UpdateDishRequest = {
    name,
    category_id: params.formData.category_id,
    price: params.formData.price,
    is_online: params.formData.is_online,
    is_available: true,
    is_packaging: params.formData.is_packaging,
    prepare_time: params.formData.prepare_time
  }

  if (description) {
    payload.description = description
  }
  if (params.formData.image_asset_id) {
    payload.image_asset_id = params.formData.image_asset_id
  }
  if (params.selectedDishTagIds.length > 0 || params.isEdit) {
    payload.tag_ids = params.selectedDishTagIds
  }
  if (params.isEdit || params.formData.member_price > 0) {
    payload.member_price = params.formData.member_price
  }

  return payload
}

export function buildDishFeaturedTags(params: { isFeatured: boolean, isHotSelling: boolean }): string[] {
  const featuredTags: string[] = []
  if (params.isFeatured) {
    featuredTags.push('推荐')
  }
  if (params.isHotSelling) {
    featuredTags.push('热卖')
  }
  return featuredTags
}