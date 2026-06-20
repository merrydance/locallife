import {
  MerchantPackagingOrderType,
  MerchantPackagingOptionResponse,
  MerchantPackagingRequestContext,
  MerchantPackagingService,
  MerchantPackagingSettingsResponse,
  UpsertMerchantPackagingOptionRequest
} from '../_main_shared/api/packaging'
import { getErrorUserMessage } from '../../../utils/user-facing'

export interface PackagingOptionDraft {
  local_id: string
  id: number
  name: string
  description: string
  price_yuan: string
  is_enabled: boolean
  sort_order: number
  deleted: boolean
}

export interface PackagingSettingsDraft {
  enabled: boolean
  required: boolean
  applicable_order_types: MerchantPackagingOrderType[]
  default_option_id: number
  options: PackagingOptionDraft[]
}

export interface PackagingOrderTypeOption {
  key: MerchantPackagingOrderType
  label: string
  desc: string
}

interface OptionSaveMapping {
  previousId: number
  savedId: number
  enabled: boolean
  deleted: boolean
}

interface PackagingPagePatch {
  saving: boolean
  hasChanges: boolean
  form: PackagingSettingsDraft
  saveErrorMessage: string
}

export const ORDER_TYPE_OPTIONS: PackagingOrderTypeOption[] = [
  { key: 'takeout', label: '外卖配送', desc: '顾客选择配送到地址时使用' },
  { key: 'takeaway', label: '到店自取', desc: '顾客下单后到店自取时使用' }
]

export const PACKAGING_AUTO_REFRESH_WINDOW_MS = 60 * 1000
export const DEFAULT_ORDER_TYPES: MerchantPackagingOrderType[] = ['takeout', 'takeaway']

export function clonePackagingDraft<T>(value: T): T {
  return JSON.parse(JSON.stringify(value))
}

export function shouldRefreshPackagingSettings(lastLoadedAt: number, freshnessWindowMs: number): boolean {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

export function hasPackagingDraftChanged(current: PackagingSettingsDraft, initial: PackagingSettingsDraft): boolean {
  return JSON.stringify(current) !== JSON.stringify(initial)
}

export function normalizePackagingPriceYuan(value: string): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''

  const amount = Number(trimmed)
  if (!Number.isFinite(amount)) return trimmed

  return amount === 0 ? '0' : amount.toFixed(2)
}

export function buildPackagingDraft(
  settings: MerchantPackagingSettingsResponse,
  options: MerchantPackagingOptionResponse[]
): PackagingSettingsDraft {
  return {
    enabled: Boolean(settings.enabled),
    required: Boolean(settings.required),
    applicable_order_types: normalizeOrderTypes(settings.applicable_order_types || DEFAULT_ORDER_TYPES),
    default_option_id: settings.default_option_id || 0,
    options: (options || []).map((option, index) => ({
      local_id: option.id ? `option_${option.id}` : `new_${Date.now()}_${index}`,
      id: option.id,
      name: option.name || '',
      description: option.description || '',
      price_yuan: priceCentsToYuan(option.price),
      is_enabled: Boolean(option.is_enabled),
      sort_order: Number.isFinite(option.sort_order) ? option.sort_order : index,
      deleted: false
    }))
  }
}

export function createBlankPackagingOption(index: number): PackagingOptionDraft {
  return {
    local_id: `new_${Date.now()}_${index}`,
    id: 0,
    name: '',
    description: '',
    price_yuan: '0',
    is_enabled: true,
    sort_order: index,
    deleted: false
  }
}

export function replacePackagingOptionAt(
  options: PackagingOptionDraft[],
  localId: string,
  updater: (option: PackagingOptionDraft) => PackagingOptionDraft
): PackagingOptionDraft[] {
  return options.map((option) => option.local_id === localId ? updater(option) : option)
}

export function validatePackagingDraft(form: PackagingSettingsDraft): void {
  const activeOptions = getActiveOptions(form)

  if (form.enabled && form.applicable_order_types.length === 0) {
    throw new Error('请至少选择一个适用订单类型')
  }
  if (form.enabled && activeOptions.length === 0) {
    throw new Error('请至少添加一个包装方式')
  }
  if (form.enabled && form.required && getEnabledActiveOptions(form).length === 0) {
    throw new Error('必选包装需要至少一个启用的包装方式')
  }

  const seenNames = new Set<string>()
  for (const option of activeOptions) {
    const name = normalizeOptionName(option.name)
    if (!name) throw new Error('包装名称不能为空')
    if (name.length > 50) throw new Error('包装名称不能超过50个字符')
    if (String(option.description || '').trim().length > 200) throw new Error('包装说明不能超过200个字符')
    const nameKey = normalizeOptionNameKey(name)
    if (seenNames.has(nameKey)) throw new Error('包装名称不能重复')

    seenNames.add(nameKey)
    parsePriceCents(option.price_yuan)
  }

  if (form.default_option_id > 0) {
    const defaultOption = activeOptions.find((option) => option.id === form.default_option_id)
    if (defaultOption && !defaultOption.is_enabled) throw new Error('默认包装方式需保持启用')
  }
}

export function buildPackagingSaveFailurePatch(
  err: unknown,
  currentForm: PackagingSettingsDraft
): PackagingPagePatch {
  const message = getErrorUserMessage(err, '保存失败，请检查网络后重试')
  return {
    saving: false,
    hasChanges: true,
    form: clonePackagingDraft(currentForm),
    saveErrorMessage: message.includes('保存失败') ? message : `保存失败，${message}`
  }
}

export async function savePackagingDraft(
  form: PackagingSettingsDraft,
  context?: MerchantPackagingRequestContext
): Promise<PackagingSettingsDraft> {
  try {
    return await savePackagingDraftInternal(form, context)
  } catch (err) {
    await reconcilePersistedOptionIds(form, context)
    throw err
  }
}

async function savePackagingDraftInternal(
  form: PackagingSettingsDraft,
  context?: MerchantPackagingRequestContext
): Promise<PackagingSettingsDraft> {
  const activeOptions = getActiveOptions(form)
  const activePersistedEnabledIds = new Set(
    activeOptions
      .filter((option) => option.id > 0 && option.is_enabled)
      .map((option) => option.id)
  )
  const firstPassEnabled = form.enabled ? activePersistedEnabledIds.size > 0 : false
  const firstPassDefaultId = firstPassEnabled && activePersistedEnabledIds.has(form.default_option_id)
    ? form.default_option_id
    : 0

  await MerchantPackagingService.updateSettings({
    enabled: firstPassEnabled,
    required: form.required,
    applicable_order_types: form.applicable_order_types,
    default_option_id: firstPassDefaultId || null
  }, context)

  const mappings: OptionSaveMapping[] = []
  let nextSortOrder = 0

  for (const option of form.options || []) {
    const previousId = option.id
    if (option.deleted) {
      if (option.id > 0) await MerchantPackagingService.deleteOption(option.id, context)
      mappings.push({ previousId: option.id, savedId: option.id, enabled: false, deleted: true })
      continue
    }

    const sortOrder = nextSortOrder
    const saved = option.id > 0
      ? await MerchantPackagingService.updateOption(option.id, toOptionPayload(option, sortOrder), context)
      : await MerchantPackagingService.createOption(toOptionPayload(option, sortOrder), context)
    applySavedOption(option, saved, sortOrder)
    nextSortOrder += 1
    mappings.push({ previousId, savedId: saved.id, enabled: saved.is_enabled, deleted: false })
  }

  const finalDefaultId = resolveDefaultAfterSave(form, mappings)
  form.default_option_id = finalDefaultId
  if (form.enabled !== firstPassEnabled || finalDefaultId !== firstPassDefaultId) {
    await MerchantPackagingService.updateSettings({
      enabled: form.enabled,
      required: form.required,
      applicable_order_types: form.applicable_order_types,
      default_option_id: finalDefaultId || null
    }, context)
  }

  return clonePackagingDraft(form)
}

function normalizeOrderTypes(values?: string[] | null): MerchantPackagingOrderType[] {
  const selected = new Set<MerchantPackagingOrderType>()
  for (const value of values || []) {
    if (value === 'takeout' || value === 'takeaway') selected.add(value)
  }
  return DEFAULT_ORDER_TYPES.filter((item) => selected.has(item))
}

function normalizeOptionName(name: string): string {
  return String(name || '').trim()
}

function normalizeOptionNameKey(name: string): string {
  return normalizeOptionName(name).toLowerCase()
}

function parsePriceCents(value: string): number {
  const trimmed = String(value || '').trim()
  if (!/^\d+(\.\d{0,2})?$/.test(trimmed)) {
    throw new Error('包装费用需为非负金额，最多两位小数')
  }

  const amount = Number(trimmed)
  if (!Number.isFinite(amount) || amount < 0) {
    throw new Error('包装费用需为非负金额，最多两位小数')
  }

  const cents = Math.round(amount * 100)
  if (cents > 9999900) throw new Error('单个包装费用不能超过99999元')
  return cents
}

function priceCentsToYuan(price: number): string {
  return Number.isFinite(price) && price > 0 ? (price / 100).toFixed(2) : '0'
}

function toOptionPayload(option: PackagingOptionDraft, sortOrder: number): UpsertMerchantPackagingOptionRequest {
  return {
    name: normalizeOptionName(option.name),
    description: String(option.description || '').trim(),
    price: parsePriceCents(option.price_yuan),
    is_enabled: Boolean(option.is_enabled),
    sort_order: sortOrder
  }
}

function applySavedOption(
  option: PackagingOptionDraft,
  saved: MerchantPackagingOptionResponse,
  fallbackSortOrder: number
) {
  option.id = saved.id
  option.local_id = saved.id > 0 ? `option_${saved.id}` : option.local_id
  option.name = saved.name || normalizeOptionName(option.name)
  option.description = saved.description || ''
  option.price_yuan = priceCentsToYuan(saved.price)
  option.is_enabled = Boolean(saved.is_enabled)
  option.sort_order = Number.isFinite(saved.sort_order) ? saved.sort_order : fallbackSortOrder
  option.deleted = false
}

async function reconcilePersistedOptionIds(
  form: PackagingSettingsDraft,
  context?: MerchantPackagingRequestContext
): Promise<void> {
  try {
    const persistedOptions = await MerchantPackagingService.listOptions(context)
    const persistedByName = new Map<string, MerchantPackagingOptionResponse>()

    for (const option of persistedOptions || []) {
      const key = normalizeOptionNameKey(option.name)
      if (key && option.id > 0 && !persistedByName.has(key)) {
        persistedByName.set(key, option)
      }
    }

    for (const option of form.options || []) {
      if (option.id > 0 || option.deleted) continue

      const persisted = persistedByName.get(normalizeOptionNameKey(option.name))
      if (!persisted) continue

      option.id = persisted.id
      option.local_id = `option_${persisted.id}`
    }
  } catch (_err) {
    // Best-effort recovery: the original save error remains the user-visible failure.
  }
}

function getActiveOptions(form: PackagingSettingsDraft): PackagingOptionDraft[] {
  return (form.options || []).filter((option) => !option.deleted)
}

function getEnabledActiveOptions(form: PackagingSettingsDraft): PackagingOptionDraft[] {
  return getActiveOptions(form).filter((option) => option.is_enabled)
}

function resolveDefaultAfterSave(form: PackagingSettingsDraft, mappings: OptionSaveMapping[]): number {
  if (!form.default_option_id) return 0

  const directMapping = mappings.find((item) => item.previousId === form.default_option_id || item.savedId === form.default_option_id)
  if (directMapping && directMapping.enabled && !directMapping.deleted) return directMapping.savedId

  const defaultDraft = getEnabledActiveOptions(form).find((option) => option.id === form.default_option_id)
  return defaultDraft ? defaultDraft.id : 0
}
