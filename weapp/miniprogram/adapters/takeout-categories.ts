import type { ActiveCategory } from '../api/location'

export interface TakeoutCategoryGridItem {
  id: string
  name: string
  description: string
  iconText: string
  iconStyle: string
  selected: boolean
}

const CATEGORY_TONES = [
  { background: 'linear-gradient(135deg, #fff1df 0%, #ffe3bd 100%)', color: '#b65b00' },
  { background: 'linear-gradient(135deg, #e8f7ef 0%, #cff0dc 100%)', color: '#146c43' },
  { background: 'linear-gradient(135deg, #e9f3ff 0%, #d8e8ff 100%)', color: '#0f5cab' },
  { background: 'linear-gradient(135deg, #fff0ef 0%, #ffd9d4 100%)', color: '#b33f32' },
  { background: 'linear-gradient(135deg, #f4f0ff 0%, #e4dbff 100%)', color: '#5d3ba8' },
  { background: 'linear-gradient(135deg, #eef6ff 0%, #dbeeff 100%)', color: '#0b5fa5' },
  { background: 'linear-gradient(135deg, #fff7dc 0%, #ffeab0 100%)', color: '#9f6500' },
  { background: 'linear-gradient(135deg, #eefaf7 0%, #d9f3ec 100%)', color: '#12685a' }
]

const ALL_CATEGORY_STYLE = 'background: linear-gradient(135deg, #2f3640 0%, #4b5563 100%); color: #ffffff;'

function formatMerchantCount(merchantCount: number, isAll: boolean): string {
  if (merchantCount <= 0) {
    return isAll ? '附近优选' : '附近在售'
  }

  return `${merchantCount}家在售`
}

function buildIconText(name: string): string {
  const compactName = name.trim()
  if (!compactName) {
    return '类目'
  }

  return compactName.length <= 2 ? compactName : compactName.slice(0, 2)
}

function buildToneStyle(categoryId: number): string {
  const tone = CATEGORY_TONES[Math.abs(categoryId) % CATEGORY_TONES.length]
  return `background: ${tone.background}; color: ${tone.color};`
}

export function buildTakeoutCategoryGridItems(
  categories: ActiveCategory[],
  activeCategoryId: string
): TakeoutCategoryGridItem[] {
  const totalMerchantCount = categories.reduce((sum, category) => sum + (category.merchant_count || 0), 0)
  const normalizedActiveCategoryId = activeCategoryId || ''

  const allItem: TakeoutCategoryGridItem = {
    id: '',
    name: '全部',
    description: formatMerchantCount(totalMerchantCount, true),
    iconText: '全部',
    iconStyle: ALL_CATEGORY_STYLE,
    selected: normalizedActiveCategoryId === ''
  }

  const categoryItems = categories.map((category) => ({
    id: String(category.id),
    name: category.name,
    description: formatMerchantCount(category.merchant_count, false),
    iconText: buildIconText(category.name),
    iconStyle: buildToneStyle(category.id),
    selected: normalizedActiveCategoryId === String(category.id)
  }))

  return [allItem, ...categoryItems]
}