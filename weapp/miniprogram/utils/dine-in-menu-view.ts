import { getPublicImageUrl } from './image'
import { formatPriceNoSymbol } from './util'
import {
  loadDineInSessionMenu,
  loadReservationMenuSource,
  type MenuCart,
  type MenuCartItem,
  type MenuCategoryInfo,
  type MenuComboInfo,
  type MenuCustomizationGroup,
  type MenuDishInfo,
  type MenuMerchantInfo,
  type MenuPromotionInfo,
  type MenuPublicDish,
  type MenuTableInfo
} from '../services/dine-in-menu'

export type MenuDish = {
  id: number
  merchant_id: number
  name: string
  description?: string
  price: number
  member_price?: number | null
  image_url: string
  is_available: boolean
  is_online: boolean
  category_id?: number
  category_name?: string
  sort_order: number
  tags: string[]
  customization_groups: MenuCustomizationGroup[]
  priceDisplay: string
  memberPriceDisplay: string | null
  hasCustomizations: boolean
  cartQty: number
}

export type MenuCategory = {
  id: number
  name: string
  sort_order?: number
  dishes: MenuDish[]
}

export type CartItemView = MenuCartItem & {
  priceDisplay: string
  subtotalDisplay: string
}

export type CartView = MenuCart & {
  total_quantity: number
  subtotalDisplay: string
  items: CartItemView[]
}

export type MerchantInfoView = MenuMerchantInfo | { id: number, name: string, logo_url?: string }
export type TableInfoView = MenuTableInfo | { table_no: string }

export type DrawerDish = MenuDish & {
  spec_groups?: Array<{
    id: string
    name: string
    is_required: boolean
    specs: Array<{
      id: string
      name: string
      price_diff: number
      priceDiffDisplay: string | null
    }>
  }>
}

type SessionMenuResponse = Awaited<ReturnType<typeof loadDineInSessionMenu>>
type ReservationMenuSource = Awaited<ReturnType<typeof loadReservationMenuSource>>

function buildSessionDishView(dish: MenuDishInfo): MenuDish {
  return {
    ...dish,
    image_url: getPublicImageUrl(dish.image_url || ''),
    priceDisplay: formatPriceNoSymbol(dish.price || 0),
    memberPriceDisplay: dish.member_price ? formatPriceNoSymbol(dish.member_price) : null,
    hasCustomizations: Array.isArray(dish.customization_groups) && dish.customization_groups.length > 0,
    cartQty: 0,
    tags: dish.tags || [],
    customization_groups: dish.customization_groups || []
  }
}

function buildReservationDishView(dish: MenuPublicDish, merchantId: number): MenuDish {
  return {
    id: dish.id,
    merchant_id: merchantId,
    name: dish.name,
    description: dish.description,
    price: dish.price,
    member_price: dish.member_price,
    image_url: getPublicImageUrl(dish.image_url || ''),
    is_available: true,
    is_online: true,
    category_id: dish.category_id,
    category_name: dish.category_name,
    sort_order: 0,
    tags: dish.tags || [],
    customization_groups: dish.customization_groups || [],
    priceDisplay: formatPriceNoSymbol(dish.price || 0),
    memberPriceDisplay: dish.member_price ? formatPriceNoSymbol(dish.member_price) : null,
    hasCustomizations: Array.isArray(dish.customization_groups) && dish.customization_groups.length > 0,
    cartQty: 0
  }
}

export function buildSessionMenuState(menuResponse: SessionMenuResponse) {
  const allDishes: MenuDish[] = []
  const processedCategories: MenuCategory[] = (menuResponse.categories || []).map((cat: MenuCategoryInfo) => {
    const dishes = (cat.dishes || []).map((dish: MenuDishInfo) => {
      const processedDish = buildSessionDishView(dish)
      allDishes.push(processedDish)
      return processedDish
    })
    return { id: cat.id, name: cat.name, sort_order: cat.sort_order, dishes }
  })

  return {
    title: menuResponse.merchant.name,
    state: {
      sessionId: menuResponse.session.id,
      billingGroupId: menuResponse.billing_group.id,
      tableId: menuResponse.session.table_id,
      merchantId: menuResponse.session.merchant_id,
      reservationId: menuResponse.session.reservation_id || 0,
      orderType: 'dine_in' as const,
      tableNo: menuResponse.table.table_no,
      merchantInfo: menuResponse.merchant,
      tableInfo: menuResponse.table,
      categories: [{ id: 0, name: '全部', sort_order: -1, dishes: allDishes }, ...processedCategories],
      combos: menuResponse.combos || [] as MenuComboInfo[],
      promotions: menuResponse.promotions || [] as MenuPromotionInfo[],
      currentCategoryId: 0,
      currentDishes: allDishes,
      loading: false
    }
  }
}

export function buildReservationMenuState(reservationId: number, merchantId: number, source: ReservationMenuSource) {
  const tableNo = source.reservation.table_no
  if (!tableNo) {
    throw new Error('预订信息缺少桌台号')
  }

  const dishes = (source.dishesResponse.dishes || []).map((dish: MenuPublicDish) => buildReservationDishView(dish, merchantId))
  const categoryMap = new Map<number, MenuCategory>()

  dishes.forEach((dish) => {
    const categoryId = dish.category_id || 0
    const categoryName = dish.category_name || '其他'
    if (!categoryMap.has(categoryId)) {
      categoryMap.set(categoryId, { id: categoryId, name: categoryName, dishes: [] })
    }
    categoryMap.get(categoryId)!.dishes.push(dish)
  })

  const merchantName = source.merchantDetail.name
  if (!merchantName) {
    throw new Error('无法获取商户信息')
  }

  return {
    title: merchantName,
    state: {
      reservationId,
      merchantId,
      tableNo,
      merchantInfo: {
        id: merchantId,
        name: merchantName
      } as MerchantInfoView,
      tableInfo: {
        table_no: tableNo
      } as TableInfoView,
      categories: [
        { id: 0, name: '全部', sort_order: -1, dishes: [...dishes] },
        ...Array.from(categoryMap.values()).sort((left, right) => left.id - right.id)
      ],
      currentCategoryId: 0,
      currentDishes: dishes,
      loading: false
    }
  }
}

function buildCartQtyMap(items: CartItemView[]) {
  const cartQtyMap = new Map<number, number>()
  items.forEach((item) => {
    if (item.dish_id) {
      cartQtyMap.set(item.dish_id, (cartQtyMap.get(item.dish_id) || 0) + item.quantity)
    }
  })
  return cartQtyMap
}

export function buildMenuCartView(cart: MenuCart): CartView {
  return {
    ...cart,
    total_quantity: cart.total_count || 0,
    subtotalDisplay: formatPriceNoSymbol(cart.subtotal || 0),
    items: (cart.items || []).map((item: MenuCartItem) => ({
      ...item,
      image_url: getPublicImageUrl(item.image_url),
      priceDisplay: formatPriceNoSymbol(item.unit_price || 0),
      subtotalDisplay: formatPriceNoSymbol(item.subtotal || (item.unit_price || 0) * (item.quantity || 1))
    }))
  }
}

export function buildMenuCartDataUpdate(params: {
  cart: MenuCart
  currentDishes: MenuDish[]
  categories: MenuCategory[]
}) {
  const processedCart = buildMenuCartView(params.cart)
  const cartQtyMap = buildCartQtyMap(processedCart.items)
  const dataUpdate: WechatMiniprogram.IAnyObject = {
    cart: processedCart,
    cartTotal: params.cart.subtotal,
    cartCount: params.cart.total_count,
    totalPrice: params.cart.subtotal,
    totalCount: params.cart.total_count
  }

  params.currentDishes.forEach((dish, index) => {
    const nextQty = cartQtyMap.get(dish.id) || 0
    if (dish.cartQty !== nextQty) {
      dataUpdate[`currentDishes[${index}].cartQty`] = nextQty
    }
  })

  params.categories.forEach((category, categoryIndex) => {
    (category.dishes || []).forEach((dish, dishIndex) => {
      const nextQty = cartQtyMap.get(dish.id) || 0
      if (dish.cartQty !== nextQty) {
        dataUpdate[`categories[${categoryIndex}].dishes[${dishIndex}].cartQty`] = nextQty
      }
    })
  })

  return dataUpdate
}

export function buildDishQtyUpdate(params: {
  currentDishes: MenuDish[]
  categories: MenuCategory[]
  dishId: number
  nextQty: number
}) {
  const dataUpdate: WechatMiniprogram.IAnyObject = {}

  params.currentDishes.forEach((dish, index) => {
    if (dish.id === params.dishId && dish.cartQty !== params.nextQty) {
      dataUpdate[`currentDishes[${index}].cartQty`] = params.nextQty
    }
  })

  params.categories.forEach((category, categoryIndex) => {
    (category.dishes || []).forEach((dish, dishIndex) => {
      if (dish.id === params.dishId && dish.cartQty !== params.nextQty) {
        dataUpdate[`categories[${categoryIndex}].dishes[${dishIndex}].cartQty`] = params.nextQty
      }
    })
  })

  return dataUpdate
}

export function findPlainDishCartItem(cart: CartView | MenuCart | null | undefined, dishId: number) {
  return cart?.items.find((item) => {
    const hasCustomizations = item.customizations && Object.keys(item.customizations).length > 0
    return item.dish_id === dishId && !hasCustomizations
  })
}

export function buildOptimisticCartItemUpdate(params: {
  cart: CartView | null
  itemId: number
  nextQty: number
  currentDishes: MenuDish[]
  categories: MenuCategory[]
}) {
  if (!params.cart) {
    return null
  }

  const items = [...(params.cart.items || [])] as CartItemView[]
  const index = items.findIndex((item) => item.id === params.itemId)
  if (index < 0) {
    return null
  }

  const target = items[index]
  const unitPrice = target.unit_price || (target as { price?: number }).price || 0
  const previousQty = target.quantity || 0
  const safeNextQty = Math.max(0, params.nextQty)

  if (safeNextQty <= 0) {
    items.splice(index, 1)
  } else {
    items[index] = {
      ...target,
      quantity: safeNextQty,
      subtotal: unitPrice * safeNextQty,
      subtotalDisplay: formatPriceNoSymbol(unitPrice * safeNextQty)
    } as CartItemView
  }

  const nextTotalCount = Math.max(0, (params.cart.total_count || 0) - previousQty + safeNextQty)
  const nextSubtotal = Math.max(0, (params.cart.subtotal || 0) - unitPrice * previousQty + unitPrice * safeNextQty)
  const dataUpdate: WechatMiniprogram.IAnyObject = {
    'cart.items': items,
    'cart.total_count': nextTotalCount,
    'cart.total_quantity': nextTotalCount,
    'cart.subtotal': nextSubtotal,
    'cart.subtotalDisplay': formatPriceNoSymbol(nextSubtotal),
    cartTotal: nextSubtotal,
    cartCount: nextTotalCount,
    totalPrice: nextSubtotal,
    totalCount: nextTotalCount
  }

  if (target.dish_id) {
    const nextDishQty = items
      .filter((item) => item.dish_id === target.dish_id)
      .reduce((sum, item) => sum + (item.quantity || 0), 0)
    Object.assign(dataUpdate, buildDishQtyUpdate({
      currentDishes: params.currentDishes,
      categories: params.categories,
      dishId: target.dish_id,
      nextQty: nextDishQty
    }))
  }

  return dataUpdate
}

export function buildOptimisticDishDeltaUpdate(params: {
  cart: CartView | null
  currentDishes: MenuDish[]
  categories: MenuCategory[]
  dishId: number
  deltaQty: number
}) {
  if (!params.cart) {
    return null
  }

  const dish = params.currentDishes.find((item) => item.id === params.dishId)
  const unitPrice = dish?.price || 0
  const currentQty = dish?.cartQty || 0
  const nextQty = Math.max(0, currentQty + params.deltaQty)
  const plainDishItem = findPlainDishCartItem(params.cart, params.dishId)

  if (plainDishItem) {
    return buildOptimisticCartItemUpdate({
      cart: params.cart,
      itemId: plainDishItem.id,
      nextQty: Math.max(0, (plainDishItem.quantity || 0) + params.deltaQty),
      currentDishes: params.currentDishes,
      categories: params.categories
    })
  }

  const nextTotalCount = Math.max(0, (params.cart.total_count || 0) + params.deltaQty)
  const nextSubtotal = Math.max(0, (params.cart.subtotal || 0) + unitPrice * params.deltaQty)
  return {
    'cart.total_count': nextTotalCount,
    'cart.total_quantity': nextTotalCount,
    'cart.subtotal': nextSubtotal,
    'cart.subtotalDisplay': formatPriceNoSymbol(nextSubtotal),
    cartTotal: nextSubtotal,
    cartCount: nextTotalCount,
    totalPrice: nextSubtotal,
    totalCount: nextTotalCount,
    ...buildDishQtyUpdate({
      currentDishes: params.currentDishes,
      categories: params.categories,
      dishId: params.dishId,
      nextQty
    })
  }
}

export function buildDrawerState(dish: MenuDish) {
  const spec_groups = (dish.customization_groups || []).map((group) => ({
    id: String(group.id),
    name: group.name,
    is_required: group.is_required,
    specs: (group.options || []).map((option) => ({
      id: String(option.id),
      name: option.tag_name || '选项',
      price_diff: option.extra_price || 0,
      priceDiffDisplay: option.extra_price ? formatPriceNoSymbol(option.extra_price) : null
    }))
  }))

  const drawerSpecs: Record<string, string> = {}
  spec_groups.forEach((group) => {
    if (group.is_required && group.specs.length > 0) {
      drawerSpecs[group.id] = group.specs[0].id
    }
  })

  return {
    drawerDish: {
      ...dish,
      spec_groups
    } as DrawerDish,
    drawerSpecs,
    drawerQty: 1,
    drawerVisible: true
  }
}

export function validateDrawerSelection(drawerDish: DrawerDish | null, drawerSpecs: Record<string, string>) {
  if (!drawerDish) {
    return '菜品信息已失效'
  }

  for (const group of drawerDish.spec_groups || []) {
    if (group.is_required && !drawerSpecs[group.id]) {
      return `请选择${group.name}`
    }
  }

  return ''
}