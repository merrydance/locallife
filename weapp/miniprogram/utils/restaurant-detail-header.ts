const RESTAURANT_HEADER_COLLAPSE_SCROLL_TOP = 80
const RESTAURANT_HEADER_EXPAND_SCROLL_TOP = 20

export type RestaurantHeaderScrollDirection = 'up' | 'down' | 'none'

export interface RestaurantHeaderCollapseInput {
  scrollTop: number
  headerCollapsed: boolean
  scrollDirection: RestaurantHeaderScrollDirection
}

export function resolveRestaurantHeaderCollapsed(input: RestaurantHeaderCollapseInput): boolean {
  const scrollTop = Number.isFinite(input.scrollTop) ? input.scrollTop : 0

  if (input.headerCollapsed) {
    return !(input.scrollDirection === 'down' && scrollTop <= RESTAURANT_HEADER_EXPAND_SCROLL_TOP)
  }

  return input.scrollDirection === 'up' || scrollTop >= RESTAURANT_HEADER_COLLAPSE_SCROLL_TOP
}
