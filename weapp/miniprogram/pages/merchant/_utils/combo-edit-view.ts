import {
  ComboDishInput,
  ComboSetWithDetailsResponse,
  CustomizationGroup,
  DishResponse,
  TagInfo
} from '../_main_shared/api/dish'
import { getPublicImageUrl } from '../../../utils/image'

export interface DishOption {
  id: number
  name: string
  price: number
  is_available: boolean
  is_online: boolean
  image_url: string
  checked: boolean
  quantity: number
  customization_groups: CustomizationGroup[]
  customization_error_message: string
  customizations: Record<string, number | string>
  customization_summary: string
  customization_extra_price: number
}

export interface ComboEditOptions {
  id?: string
}

export interface FormInputDetail {
  value: string
}

export type SelectedSpecState = Record<string, string>

export interface SelectedComboDishState {
  quantity: number
  customizations: Record<string, string>
  customizationSummary: string
  customizationExtraPrice: number
}

export function normalizeDishQuantity(quantity?: number): number {
  if (!Number.isFinite(quantity) || !quantity) {
    return 1
  }

  const safeQuantity = Math.round(quantity)
  if (safeQuantity < 1) {
    return 1
  }
  if (safeQuantity > 99) {
    return 99
  }
  return safeQuantity
}

export function buildSelectedComboDishes(dishes: DishOption[]): ComboDishInput[] {
  return dishes
    .filter((dish) => dish.checked)
    .map((dish) => {
      const comboDish: ComboDishInput = {
        dish_id: dish.id,
        quantity: normalizeDishQuantity(dish.quantity)
      }

      if (Object.keys(dish.customizations || {}).length > 0) {
        comboDish.customizations = dish.customizations
      }

      return comboDish
    })
}

export function normalizeComboCustomizations(customizations?: Record<string, unknown> | null): Record<string, string> {
  const normalized: Record<string, string> = {}

  if (!customizations || typeof customizations !== 'object') {
    return normalized
  }

  Object.entries(customizations).forEach(([key, value]) => {
    if (key === 'meta_specs') {
      if (typeof value === 'string' && value.trim()) {
        normalized[key] = value.trim()
      }
      return
    }

    if (typeof value === 'number' || typeof value === 'string') {
      normalized[key] = String(value)
    }
  })

  return normalized
}

export function buildSelectedSpecState(groups: CustomizationGroup[], customizations: Record<string, number | string>) {
  const selectedSpecs: SelectedSpecState = {}
  let hasInvalidSelection = false

  groups.forEach((group) => {
    const rawValue = customizations[String(group.id)]
    const existingSpecId = rawValue ? String(rawValue) : ''
    const matchedOption = (group.options || []).find((option) => String(option.id) === existingSpecId)

    if (matchedOption) {
      selectedSpecs[String(group.id)] = String(matchedOption.id)
      return
    }

    if (existingSpecId) {
      hasInvalidSelection = true
    }

    if (group.is_required && Array.isArray(group.options) && group.options.length > 0) {
      selectedSpecs[String(group.id)] = String(group.options[0].id)
    }
  })

  return { selectedSpecs, hasInvalidSelection }
}

export function buildDishCustomizationPayload(groups: CustomizationGroup[], selectedSpecs: SelectedSpecState) {
  const customizations: Record<string, number | string> = {}
  const specNames: string[] = []
  let extraPrice = 0

  groups.forEach((group) => {
    const selectedSpecId = selectedSpecs[String(group.id)]
    if (!selectedSpecId) {
      return
    }

    const option = (group.options || []).find((candidate) => String(candidate.id) === selectedSpecId)
    if (!option) {
      return
    }

    customizations[String(group.id)] = String(option.id)
    specNames.push(option.tag_name)
    extraPrice += option.extra_price || 0
  })

  const summary = specNames.join(' / ')
  if (summary) {
    customizations.meta_specs = summary
  }

  return {
    customizations,
    summary,
    extraPrice
  }
}

export function syncDishCustomizationSelection(dish: DishOption): DishOption {
  if (!dish.checked || !Array.isArray(dish.customization_groups) || dish.customization_groups.length === 0) {
    return {
      ...dish,
      customization_error_message: ''
    }
  }

  const { selectedSpecs, hasInvalidSelection } = buildSelectedSpecState(dish.customization_groups, dish.customizations || {})
  const payload = buildDishCustomizationPayload(dish.customization_groups, selectedSpecs)

  return {
    ...dish,
    customizations: payload.customizations,
    customization_summary: payload.summary,
    customization_extra_price: payload.extraPrice,
    customization_error_message: hasInvalidSelection ? '已保存规格已变化，已按当前规格重置。' : ''
  }
}

export function buildSelectedTagState(selectedTagIds: number[]): Record<string, boolean> {
  return selectedTagIds.reduce<Record<string, boolean>>((result, id) => {
    result[String(id)] = true
    return result
  }, {})
}

export function formatFenToYuanInput(value?: number): string {
  if (!Number.isFinite(value) || !value) {
    return ''
  }

  return (Number(value) / 100).toFixed(2)
}

export function parsePriceInputToFen(value: string): number {
  const parsed = Number.parseFloat((value || '').trim())
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }

  return Math.round(parsed * 100)
}

export function buildDishOptions(dishes: DishResponse[]): DishOption[] {
  return dishes.map((dish) => ({
    id: dish.id,
    name: dish.name,
    price: dish.price,
    is_available: dish.is_available,
    is_online: dish.is_online,
    image_url: getPublicImageUrl(dish.image_url || ''),
    checked: false,
    quantity: 1,
    customization_groups: Array.isArray(dish.customization_groups) ? dish.customization_groups : [],
    customization_error_message: '',
    customizations: {},
    customization_summary: '',
    customization_extra_price: 0
  }))
}

export function buildSelectedDishMap(comboRes: ComboSetWithDetailsResponse | null) {
  return new Map<number, SelectedComboDishState>(
    (comboRes?.dishes || []).map((dish: ComboSetWithDetailsResponse['dishes'][number]) => [dish.dish_id, {
      quantity: normalizeDishQuantity(dish.quantity),
      customizations: normalizeComboCustomizations(dish.customizations),
      customizationSummary: dish.customization_summary || '',
      customizationExtraPrice: dish.customization_extra_price || 0
    }])
  )
}

export function buildSelectedTagIds(tags: TagInfo[] | undefined) {
  return Array.isArray(tags)
    ? tags
      .map((tag: TagInfo) => Number(tag.id))
      .filter((id: number) => Number.isFinite(id) && id > 0)
    : []
}
