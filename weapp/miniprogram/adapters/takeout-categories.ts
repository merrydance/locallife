import type { ActiveCategory } from '../api/location'

export interface TakeoutCategoryGridItem {
  id: string
  name: string
  description: string
  iconText: string
  iconStyle: string
  selected: boolean
}

const CATEGORY_EMOJI_MAP: [string, string][] = [
  ['火锅', '🍲'],
  ['砂锅', '🥘'],
  ['烧烤', '🔥'],
  ['烤', '🔥'],
  ['日料', '🍱'],
  ['寿司', '🍣'],
  ['西餐', '🍕'],
  ['披萨', '🍕'],
  ['汉堡', '🍔'],
  ['炸鸡', '🍗'],
  ['鸡', '🍗'],
  ['早餐', '🥐'],
  ['甜品', '🍰'],
  ['饮品', '🥤'],
  ['奶茶', '🧋'],
  ['咖啡', '☕'],
  ['面', '🍜'],
  ['粉', '🍜'],
  ['炒饼', '🥞'],
  ['饼', '🥞'],
  ['粥', '🥣'],
  ['海鲜', '🦐'],
  ['螃蟹', '🦀'],
  ['鱼', '🐟'],
  ['湘', '🌶️'],
  ['川', '🌶️'],
  ['辣', '🌶️'],
  ['牛', '🐄'],
  ['羊', '🐑'],
  ['猪', '🐷'],
  ['鸭', '🦆'],
  ['粤', '🥢'],
  ['广式', '🥢'],
  ['河北', '🍚'],
  ['家常', '🥘'],
  ['快餐', '🍟'],
  ['中餐', '🥡']
]

function formatMerchantCount(merchantCount: number, isAll: boolean): string {
  if (merchantCount <= 0) {
    return isAll ? '附近优选' : '附近在售'
  }

  return `${merchantCount}家在售`
}

export function buildCategoryIconEmoji(name: string): string {
  if (!name.trim()) return '🍴'
  for (const [keyword, emoji] of CATEGORY_EMOJI_MAP) {
    if (name.includes(keyword)) return emoji
  }
  return '🍴'
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
    iconText: '🍽️',
    iconStyle: '',
    selected: normalizedActiveCategoryId === ''
  }

  const categoryItems = categories.map((category) => ({
    id: String(category.id),
    name: category.name,
    description: formatMerchantCount(category.merchant_count, false),
    iconText: category.icon || buildCategoryIconEmoji(category.name),
    iconStyle: '',
    selected: normalizedActiveCategoryId === String(category.id)
  }))

  return [allItem, ...categoryItems]
}
