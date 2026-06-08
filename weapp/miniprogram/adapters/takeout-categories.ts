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
  ['麻辣烫', '🌶️'],
  ['火锅', '🍲'],
  ['砂锅', '🥘'],
  ['烧烤', '🔥'],
  ['烤串', '🔥'],
  ['串', '🔥'],
  ['烤', '🔥'],
  ['日料', '🍱'],
  ['寿司', '🍣'],
  ['刺身', '🍣'],
  ['料理', '🍱'],
  ['西餐', '🍕'],
  ['披萨', '🍕'],
  ['汉堡', '🍔'],
  ['薯条', '🍟'],
  ['热狗', '🌭'],
  ['三明治', '🥪'],
  ['塔可', '🌮'],
  ['卷饼', '🌯'],
  ['沙威玛', '🥙'],
  ['炸鸡', '🍗'],
  ['鸡排', '🍗'],
  ['鸡', '🍗'],
  ['牛排', '🥩'],
  ['烤肉', '🥩'],
  ['肉', '🥩'],
  ['早餐', '🥐'],
  ['面包', '🍞'],
  ['烘焙', '🥐'],
  ['蛋糕', '🍰'],
  ['甜品', '🧁'],
  ['甜点', '🧁'],
  ['冰淇淋', '🍦'],
  ['冰品', '🍧'],
  ['饮品', '🥤'],
  ['奶茶', '🧋'],
  ['咖啡', '☕'],
  ['茶饮', '🍵'],
  ['茶', '🍵'],
  ['果汁', '🧃'],
  ['酒', '🍺'],
  ['啤', '🍺'],
  ['米饭', '🍚'],
  ['盖饭', '🍚'],
  ['炒饭', '🍚'],
  ['饭', '🍚'],
  ['饺子', '🥟'],
  ['馄饨', '🥟'],
  ['馄飩', '🥟'],
  ['包子', '🥟'],
  ['小笼', '🥟'],
  ['面', '🍜'],
  ['粉', '🍜'],
  ['炒饼', '🥞'],
  ['饼', '🥞'],
  ['粥', '🥣'],
  ['沙拉', '🥗'],
  ['轻食', '🥗'],
  ['素食', '🥬'],
  ['海鲜', '🦐'],
  ['小龙虾', '🦞'],
  ['龙虾', '🦞'],
  ['螃蟹', '🦀'],
  ['蟹', '🦀'],
  ['生蚝', '🦪'],
  ['蚝', '🦪'],
  ['鱿鱼', '🦑'],
  ['章鱼', '🦑'],
  ['虾', '🦐'],
  ['鱼', '🐟'],
  ['湘', '🌶️'],
  ['川', '🌶️'],
  ['辣', '🌶️'],
  ['牛', '🥩'],
  ['羊', '🍖'],
  ['猪', '🥓'],
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
